package main

import "net/http"

// webhookHandler is the HTTP handler function for the /mutate endpoint.
func webhookHandler(w http.ResponseWriter, r *http.Request) {

	// Validate Request (Valid requests are POST with Content-Type: application/json)
	if !validateRequest(w, r) {
		return
	}

	// Parse the AdmissionReview request and return it.
	admissionReviewReq, err := parseRequest(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Build AdmissionReview response.
	admissionReviewResponse, err := buildResponse(w, *admissionReviewReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write the AdmissionReview response to the http response writer.
	sendResponse(w, *admissionReviewResponse)
}
