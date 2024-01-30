package main

// ServerParameters struct holds the parameters for the webhook server.
type serverParameters struct {
	httpsPort int    // https server port
	certFile  string // path to the x509 certificate for https
	keyFile   string // path to the x509 private key matching `CertFile`
}

// patchOperation is a JSON patch operation, see https://jsonpatch.com/
type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}
