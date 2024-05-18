module github.com/trilogy-group/go-diameter/v4

go 1.13

require (
	github.com/fiorix/go-diameter/v4 v4.0.4 // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.3.2
	github.com/ishidawataru/sctp v0.0.0-20190922091402-408ec287e38c
	golang.org/x/net v0.0.0-20191007182048-72f939374954
	google.golang.org/grpc v1.24.0
)

replace github.com/fiorix/go-diameter/v4 => ./