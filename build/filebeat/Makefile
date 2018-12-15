IMAGE=cargo.caicloudprivatetest.com/caicloud/filebeat:release-6.4.2-v0.0.1

.PHONY: build push

default: push

build:
	docker build -t $(IMAGE) .

push: build
	docker push $(IMAGE)
