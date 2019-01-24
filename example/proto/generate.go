package proto

// Init package directories
//go:generate mkdir -p ./embedded_docs
//go:generate mkdir -p ./go/v1
//go:generate mkdir -p ./swagger
//go:generate mkdir -p ./go/v1/mock_v1

// Generate golang packages and swagger docs
//go:generate protoc -I=/usr/local/include/google/proto -I=../vendor/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis -I=../vendor/github.com/lyft -I=../vendor/github.com/grpc-ecosystem/grpc-gateway -I=./v1 --go_out=plugins=grpc:./go/v1 --grpc-gateway_out=logtostderr=true:./go/v1 --swagger_out=logtostderr=true:./swagger --validate_out=lang=go:./go/v1 ./v1/v1.proto

// Generate embedded docs
//go:generate go run ../vendor/github.com/bdlm/grpc-gateway-wrapper/proto/vfsgen/vfsgen.go --dir=./swagger/ --outfile=./embedded_docs/embedded_docs.go --pkg=embedded_docs --variable=Docs -comment "Docs statically implements an embedded virtual filesystem provided to vfsgen.\n\tFor example, to access the v1 swagger file, use path: '/v1.swagger.json'"

// Generate mocks
//go:generate mockgen --destination=./go/v1/mock_v1/mock_v1.go github.com/bdlm/grpc-gateway-wrapper/example/proto/go/v1 K8SClient,K8SServer

// Generate PHP protobuf/grpc (done separately due to issues with php plugin)
//go:generate mkdir -p ./php/v1
//go:generate protoc -I=/usr/local/include/google/proto -I=../vendor/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis -I=../vendor/github.com/lyft -I=../vendor/github.com/grpc-ecosystem/grpc-gateway -I=./v1 --plugin=protoc-gen-grpc=/go/src/github.com/grpc/bins/opt/grpc_php_plugin --grpc_out=./php/v1 --php_out=./php/v1 ./v1/v1.proto

// Generate Typescript protobuf/grpc
//go:generate mkdir -p ./js/v1
//go:generate mkdir -p ./ts/v1
//go:generate /usr/local/bin/protoc -I=/usr/local/include/google/proto -I=../vendor/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis -I=../vendor/github.com/lyft -I=../vendor/github.com/grpc-ecosystem/grpc-gateway -I=./v1 --plugin="protoc-gen-ts=/usr/lib/node_modules/ts-protoc-gen/bin/protoc-gen-ts" --js_out=./js/v1 --ts_out=./ts/v1 ./v1/v1.proto
