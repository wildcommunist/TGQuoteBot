build:
	@go build -o bin/tgbot

run: build
	@./bin/tgbot

docker:
	@docker build -t $(docker_tag) .

upload_image: docker
	@docker push $(docker_tag)