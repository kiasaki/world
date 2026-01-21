dev:
	@which reflex>/dev/null || go install github.com/cespare/reflex@latest
	reflex -s -d none -r '.(go|js|css|html)$$' -- make run

run:
	#env AWS_BUCKET=habari-dev AWS_ACCESS_KEY=$(HABARI_AWS_ACCESS_KEY) AWS_SECRET_KEY=$(HABARI_AWS_SECRET_KEY)
	go run main.go

APP=evry
REMOTE=homelab

build:
	GOOS=linux GOARCH=amd64 go build -v -ldflags "-s -w" -installsuffix cgo -o /tmp/$(APP) main.go

deploy: build
	scp /tmp/$(APP) $(REMOTE):/tmp/aout && ssh $(REMOTE) "sudo mv /tmp/aout /usr/bin/$(APP)" && ssh $(REMOTE) "sudo systemctl restart $(APP)"
