BINARY_NAME=og
 
build:
	go build -o ${BINARY_NAME} cmd/omnigres/main.go
 
run: build
	./${BINARY_NAME}
 
clean:
	go clean
	rm -f ${BINARY_NAME}
