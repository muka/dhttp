module github.com/muka/dhttp

go 1.15

replace github.com/muka/peer => ../peer

require (
	github.com/golang/protobuf v1.4.3
	github.com/muka/peer v0.0.0
	github.com/rs/xid v1.2.1
	github.com/sirupsen/logrus v1.7.0
	github.com/stretchr/testify v1.6.1
	google.golang.org/api v0.1.0
	google.golang.org/appengine v1.4.0
)
