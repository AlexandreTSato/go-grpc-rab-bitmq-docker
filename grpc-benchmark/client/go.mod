module github.com/seuuser/grpc-benchmark/client

go 1.24.4

replace github.com/seuuser/grpc-benchmark/proto/benchmarkbp => ../proto

require google.golang.org/grpc v1.73.0

require (
	github.com/seuuser/grpc-benchmark/proto/benchmarkbp v0.0.0-00010101000000-000000000000
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250324211829-b45e905df463 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)
