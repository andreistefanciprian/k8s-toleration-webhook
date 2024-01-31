DOCKER_HUB_USERNAME := andreistefanciprian
IMAGE_NAME := k8s-toleration-webhook
DOCKER_IMAGE_NAME := $(DOCKER_HUB_USERNAME)/$(IMAGE_NAME)

build:
	docker build -t $(DOCKER_IMAGE_NAME) . -f infra/Dockerfile
	docker image push $(DOCKER_IMAGE_NAME)

template:
	helm template --namespace toleration-webhook toleration-webhook infra/toleration-webhook --create-namespace --set GoogleCASClusterIssuer.enabled=true

install:
	helm upgrade --install toleration-webhook infra/toleration-webhook --namespace toleration-webhook --create-namespace --set GoogleCASClusterIssuer.enabled=true

uninstall:
	helm uninstall toleration-webhook --namespace toleration-webhook

check:
	helm list --namespace toleration-webhook
	kubectl get MutatingWebhookConfiguration toleration-webhook --ignore-not-found=true -n toleration-webhook
	kubectl get pods,secrets,certificates -n toleration-webhook

logs:
	kubectl logs -l app.kubernetes.io/name=toleration-webhook --namespace toleration-webhook -f

unit-tests:
	go test  ./... -v