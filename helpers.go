package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"

	"k8s.io/api/admission/v1beta1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

	// Unmarshal the Deployment object from the AdmissionReview request into a Deployment struct.
	deployment := v1.Deployment{}
	err := json.Unmarshal(req.Request.Object.Raw, &deployment)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal deployment on admission request: %s", err.Error())
	}

	// Construct Deployment name in the format: namespace/name
	deploymentName := deployment.GetNamespace() + "/" + deployment.GetName()

	log.Printf("New Admission Review Request is being processed: User: %v \t Operation: %v \t Pod: %v \n",
		req.Request.UserInfo.Username,
		req.Request.Operation,
		deploymentName,
	)

	// Construct the AdmissionReview response.
	admissionReviewResponse := v1beta1.AdmissionReview{
		Response: &v1beta1.AdmissionResponse{
			UID:     req.Request.UID,
			Allowed: true,
		},
	}

	//  Check if toleration is already set
	if !tolerationExists(deployment.Spec.Template.Spec.Tolerations, toleration) {
		log.Printf("Toleration %+v does not exist in Deployment %s", toleration, deploymentName)
		patchBytes, err := buildJsonPatch(&deployment, toleration)
		if err != nil {
			return nil, fmt.Errorf("could not build JSON patch: %s", err.Error())
		}
		// admissionReviewResponse.Response.AuditAnnotations = deployment.ObjectMeta.Annotations // AuditAnnotations are added to the audit record when this admission response is added to the audit event.
		admissionReviewResponse.Response.Patch = patchBytes
		patchMsg := fmt.Sprintf("Deployment %v was updated with toleration.", deploymentName)
		stdoutMsg := fmt.Sprintf("Deployment %v does not have a toleration set.", deploymentName)
		admissionReviewResponse.Response.Warnings = []string{stdoutMsg, patchMsg}
		log.Println(patchMsg)
	} else {
		log.Printf("Toleration %v already exists in deployment %s, skipping addition", toleration, deploymentName)
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
func buildJsonPatch(deployment *v1.Deployment, toleration corev1.Toleration) ([]byte, error) {
	annotations := deployment.ObjectMeta.Annotations
	annotations["updated_by"] = "tolerationWebhook"
	tolerations := deployment.Spec.Template.Spec.Tolerations
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

// tolerationExists checks if a toleration already exists in a slice of tolerations.
func tolerationExists(existingTolerations []corev1.Toleration, toleration corev1.Toleration) bool {
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
