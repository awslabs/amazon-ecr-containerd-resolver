# Amazon ECR containerd resolver

[![Build Status](https://travis-ci.org/samuelkarp/amazon-ecr-containerd-resolver.svg?branch=master)](https://travis-ci.org/samuelkarp/amazon-ecr-containerd-resolver)

The Amazon ECR containerd resolver is an implementation of a
[containerd](https://github.com/containerd/containerd)
`Resolver`, `Fetcher`, and `Pusher` that can pull images from Amazon ECR and
push images to Amazon ECR using the Amazon ECR API instead of the Docker
Registry API.

> *Note:* This repository is a proof-of-concept and is not recommended for
> production use.

## Usage

### Pull images
```go
img, err := client.Pull(
    namespaces.NamespaceFromEnv(context.TODO()),
    "ecr.aws/arn:aws:ecr:us-west-2:123456789012:repository/myrepository:mytag",
    containerd.WithResolver(ecr.NewResolver(awsSession)),
    containerd.WithPullUnpack,
    containerd.WithSchema1Conversion)
```

### Push images
```go
ctx := namespaces.NamsepaceFromEnv(context.TODO())

img, _ := client.ImageService().Get(
	ctx,
	"docker.io/library/busybox:latest")

err = client.Push(
	ctx,
	"ecr.aws/arn:aws:ecr:us-west-2:123456789012:repository/myrepository:mytag",
	img.Target,
	containerd.WithResolver(ecr.NewResolver(awsSession)))
```

Two small example programs are provided in the [example](tree/master/example)
directory demonstrating how to use the resolver with containerd.

### `ref`

containerd specifies images with a `ref`. `ref`s are different from Docker
image names, as `ref`s intend to encode an identifier, but not a retrieval
mechanism.  `ref`s start with a DNS-style namespace that can be used to select
separate `Resolver`s to use.

The canonical `ref` format used by the amazon-ecr-containerd-resolver is 
`ecr.aws/` followed by the ARN of the repository and a label and/or a digest.

## Building

The Amazon ECR containerd resolver manages its dependencies with [Go 1.11
modules](https://github.com/golang/go/wiki/Modules) and works best when used
with Go 1.11 or greater.  If you have Go 1.11 or greater installed, you can
build the example programs with `make`.

## License

The Amazon ECR containerd resolver is licensed under the Apache 2.0 License.