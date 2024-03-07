package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"k8s.io/api/admission/v1beta1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

const (
	jsonContentType = "application/json"
)

var (
	deserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
	toleration   = corev1.Toleration{
		Key:      "SimulateNodeFailure",
		Operator: corev1.TolerationOpExists,
		Effect:   corev1.TaintEffectNoExecute,
	}
)

// parseFlags parses the CLI params and returns a ServerParameters struct.
func parseFlags() serverParameters {
	var parameters serverParameters

	// Define and parse CLI params using the "flag" package.
	flag.IntVar(&parameters.httpsPort, "httpsPort", 443, " Https server port (webhook endpoint).")
	flag.StringVar(&parameters.certFile, "tlsCertFile", "/etc/webhook/certs/tls.crt", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&parameters.keyFile, "tlsKeyFile", "/etc/webhook/certs/tls.key", "File containing the x509 private key to --tlsCertFile.")
	flag.Parse()

	return parameters
}

// validateRequest checks requests are POST with Content-Type: application/json
func validateRequest(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return false
	}

	if contentType := r.Header.Get("Content-Type"); contentType != jsonContentType {
		http.Error(w, fmt.Sprintf("Invalid content type %s", contentType), http.StatusBadRequest)
		return false
	}

	return true
}

// parseRequest parses the AdmissionReview request.
func parseRequest(w http.ResponseWriter, r *http.Request) (*v1beta1.AdmissionReview, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %s", err.Error())
	}

	var admissionReviewReq v1beta1.AdmissionReview
	if _, _, err := deserializer.Decode(body, nil, &admissionReviewReq); err != nil {
		return nil, fmt.Errorf("could not deserialize request: %s", err.Error())
	} else if admissionReviewReq.Request == nil {
		return nil, fmt.Errorf("malformed admission review (request is nil)")
	}

	// DEBUG Print string(body) when you want to see the AdmissionReview in the logs
	// log.Printf("Admission Request Body: \n %v", string(body))

	return &admissionReviewReq, nil
}

// buildResponse builds the AdmissionReview response.
func buildResponse(w http.ResponseWriter, req v1beta1.AdmissionReview) (*v1beta1.AdmissionReview, error) {
	var targetObject runtime.Object
	var resourceType string

	switch req.Request.Kind.Kind {
	case "Deployment":
		// Unmarshal the Deployment object from the AdmissionReview request into a Deployment struct.
		targetObject = &v1.Deployment{}
		resourceType = "Deployment"
	case "DaemonSet":
		// Unmarshal the DaemonSet object from the AdmissionReview request into a DaemonSet struct.
		targetObject = &v1.DaemonSet{}
		resourceType = "DaemonSet"
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", req.Request.Kind.Kind)
	}

	err := json.Unmarshal(req.Request.Object.Raw, targetObject)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal %s on admission request: %s", resourceType, err.Error())
	}

	// Construct resource name in the format: namespace/name
	resourceName := getResourceName(targetObject)

	log.Printf("New Admission Review Request is being processed: User: %v \t Operation: %v \t Pod: %v \n",
		req.Request.UserInfo.Username,
		req.Request.Operation,
		resourceName,
	)

	// Construct the AdmissionReview response.
	admissionReviewResponse := v1beta1.AdmissionReview{
		Response: &v1beta1.AdmissionResponse{
			UID:     req.Request.UID,
			Allowed: true,
		},
	}

	//  Check if toleration is already set
	if !tolerationExists(targetObject, toleration) {
		log.Printf("Toleration does not exist in %s %s", resourceType, resourceName)
		patchBytes, err := buildJsonPatch(targetObject, toleration)
		if err != nil {
			return nil, fmt.Errorf("could not build JSON patch: %s", err.Error())
		}
		// admissionReviewResponse.Response.AuditAnnotations = targetObject.ObjectMeta.Annotations // AuditAnnotations are added to the audit record when this admission response is added to the audit event.
		admissionReviewResponse.Response.Patch = patchBytes
		patchMsg := fmt.Sprintf("%s %v was updated with toleration.", resourceType, resourceName)
		stdoutMsg := fmt.Sprintf("%s %v does not have a toleration set.", resourceType, resourceName)
		admissionReviewResponse.Response.Warnings = []string{stdoutMsg, patchMsg}
		log.Println(patchMsg)
		// Increment the mutatedCounter for the given k8s object

		RecordMutatedObject(fmt.Sprintf("%v", req.Request.Operation), resourceType, strings.Split(resourceName, "/")[1], strings.Split(resourceName, "/")[0])
	} else {
		log.Printf("Toleration already exists in %s %s, skipping addition", resourceType, resourceName)
	}

	return &admissionReviewResponse, nil
}

// sendResponse writes the AdmissionReview response to the http response writer.
func sendResponse(w http.ResponseWriter, admissionReviewResponse v1beta1.AdmissionReview) {
	// Marshal the AdmissionReview response to JSON.
	bytes, err := json.Marshal(&admissionReviewResponse)
	if err != nil {
		http.Error(w, fmt.Sprintf("could not marshal JSON Admission Response: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	// Write the AdmissionReview response to the http response writer.
	w.Header().Set("Content-Type", jsonContentType)
	w.Write(bytes)
}

// buildJsonPatch builds a JSON patch to add a toleration and annotation to a Pod.
func buildJsonPatch(targetObject runtime.Object, toleration corev1.Toleration) ([]byte, error) {
	annotations := getAnnotations(targetObject)
	annotations["updated_by"] = "tolerationWebhook"

	var tolerations []corev1.Toleration
	switch obj := targetObject.(type) {
	case *v1.Deployment:
		tolerations = obj.Spec.Template.Spec.Tolerations
	case *v1.DaemonSet:
		tolerations = obj.Spec.Template.Spec.Tolerations
	default:
		return nil, fmt.Errorf("unsupported resource type for tolerations: %T", targetObject)
	}

	tolerations = append(tolerations, toleration)

	patch := []patchOperation{
		{
			Op:    "replace",
			Path:  "/spec/template/spec/tolerations",
			Value: tolerations,
		},
		{
			Op:    "replace",
			Path:  "/metadata/annotations",
			Value: annotations,
		},
	}

	// Marshal the patch slice to JSON.
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("could not marshal JSON patch: %s", err.Error())
	}

	return patchBytes, nil
}

// getAnnotations extracts and returns the annotations from the targetObject
func getAnnotations(obj runtime.Object) map[string]string {
	meta, err := meta.Accessor(obj)
	if err != nil {
		log.Printf("Error getting annotations: %v", err)
		return nil
	}
	return meta.GetAnnotations()
}

// tolerationExists checks if a toleration already exists in a slice of tolerations.
func tolerationExists(targetObject runtime.Object, toleration corev1.Toleration) bool {
	switch obj := targetObject.(type) {
	case *v1.Deployment:
		return tolerationExistsInSlice(obj.Spec.Template.Spec.Tolerations, toleration)
	case *v1.DaemonSet:
		return tolerationExistsInSlice(obj.Spec.Template.Spec.Tolerations, toleration)
	default:
		log.Printf("Unsupported resource type for toleration check: %T", targetObject)
		return false
	}
}

// tolerationExistsInSlice checks if a toleration already exists in a slice of tolerations.
func tolerationExistsInSlice(existingTolerations []corev1.Toleration, toleration corev1.Toleration) bool {
	for _, existing := range existingTolerations {
		if existing.Key == toleration.Key &&
			existing.Operator == toleration.Operator &&
			existing.Value == toleration.Value &&
			existing.Effect == toleration.Effect {
			return true
		}
	}
	return false
}

// getResourceName extracts and returns the resource name in the format: namespace/name
func getResourceName(obj runtime.Object) string {
	meta, err := meta.Accessor(obj)
	if err != nil {
		log.Printf("Error getting resource name: %v", err)
		return ""
	}
	return meta.GetNamespace() + "/" + meta.GetName()
}
