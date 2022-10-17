# Amazon ECR containerd resolver

[![.github/workflows/ci.yml](https://github.com/awslabs/amazon-ecr-containerd-resolver/actions/workflows/ci.yml/badge.svg)](https://github.com/awslabs/amazon-ecr-containerd-resolver/actions/workflows/ci.yml)
[![CodeQL Scan](https://github.com/awslabs/amazon-ecr-containerd-resolver/actions/workflows/codeql.yml/badge.svg)](https://github.com/awslabs/amazon-ecr-containerd-resolver/actions/workflows/codeql.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/awslabs/amazon-ecr-containerd-resolver)](https://goreportcard.com/report/github.com/awslabs/amazon-ecr-containerd-resolver)

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
resolver, _ := ecr.NewResolver()
img, err := client.Pull(
    namespaces.NamespaceFromEnv(context.TODO()),
    "ecr.aws/arn:aws:ecr:us-west-2:123456789012:repository/myrepository:mytag",
    containerd.WithResolver(resolver),
    containerd.WithPullUnpack,
    containerd.WithSchema1Conversion)
```

### Push images
```go
ctx := namespaces.NamespaceFromEnv(context.TODO())

img, _ := client.ImageService().Get(
	ctx,
	"docker.io/library/busybox:latest")
resolver, _ := ecr.NewResolver()
err = client.Push(
	ctx,
	"ecr.aws/arn:aws:ecr:us-west-2:123456789012:repository/myrepository:mytag",
	img.Target,
	containerd.WithResolver(resolver))
```

Two small example programs are provided in the [example](example)
directory demonstrating how to use the resolver with containerd.

### `ref`

containerd specifies images with a `ref`. `ref`s are different from Docker
image names, as `ref`s intend to encode an identifier, but not a retrieval
mechanism.  `ref`s start with a DNS-style namespace that can be used to select
separate `Resolver`s to use.

The canonical `ref` format used by the amazon-ecr-containerd-resolver is
`ecr.aws/` followed by the ARN of the repository and a label and/or a digest.

### Parallel downloads

This resolver supports request parallelization for individual layers.  This
takes advantage of HTTP [range requests](https://tools.ietf.org/html/rfc7233) to
download different parts of the same file in parallel.  This is an approach to
achieving higher throughput when [downloading from Amazon
S3](https://docs.aws.amazon.com/AmazonS3/latest/dev/optimizing-performance-design-patterns.html#optimizing-performance-parallelization),
which provides the raw blob storage for layers in Amazon ECR.

Request parallelization is not enabled by default, and the default Go HTTP
client is used instead.  To enable request parallelization, you can use the
`WithLayerDownloadParallelism` resolver option to set the amount of
parallelization per layer.

When enabled, the layer will be divided into equal-sized chunks (except for the
last chunk) and downloaded with the set amount of parallelism.  The chunks range
in size from 1 MiB to 20 MiB; anything smaller than 1 MiB will not be
parallelized and anything larger than 20 MiB * *parallelism* will use a larger
number of chunks (though only with the specified amount of parallelism).

Initial testing suggests that a parallelism setting of `4` results in 3x faster
layer downloads, but increases the amount of memory consumption between 15-20x.
Further testing is still needed.

This support is backed by the [htcat library](https://github.com/htcat/htcat).

## Building

The Amazon ECR containerd resolver manages its dependencies with [Go modules](https://github.com/golang/go/wiki/Modules) and requires Go 1.17 or greater.
If you have Go 1.17 or greater installed, you can build the example programs with `make`.

## License

The Amazon ECR containerd resolver is licensed under the Apache 2.0 License.
