BINARY_NAME=omnigres
 
build:
	go build -o ${BINARY_NAME} cmd/omnigres/main.go
 
run: build
	./${BINARY_NAME}

install: build
	mv ./${BINARY_NAME} ${HOME}/.local/bin/
 
clean:
	go clean
	rm -f ${BINARY_NAME}
