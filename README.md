# Amazon ECR containerd resolver

[![Build Status](https://travis-ci.org/samuelkarp/amazon-ecr-containerd-resolver.svg?branch=master)](https://travis-ci.org/samuelkarp/amazon-ecr-containerd-resolver)

The Amazon ECR containerd resolver is an implementation of a
[containerd](https://github.com/containerd/containerd)
`Resolver` and `Fetcher` that can pull images from Amazon ECR using the Amazon
ECR API instead of the Docker Registry API.

> *Note:* This repository is a proof-of-concept and is not recommended for
> production use.

## Usage

```go
img, err := client.Pull(
    namespaces.NamespaceFromEnv(ctx),
    "ecr.aws/arn:aws:ecr:us-west-2:123456789012:repository/myrepository:mytag",
    containerd.WithResolver(ecr.NewResolver(awsSession)),
    containerd.WithPullUnpack,
    containerd.WithSchema1Conversion)
```

A small example program is provided in the [example](tree/master/example)
directory demonstrating how to use the resolver with containerd.

### `ref`

containerd specifies images with a `ref`. `ref`s are different from Docker
image names, as `ref`s intend to encode an identifier, but not a retrieval
mechanism.  `ref`s start with a DNS-style namespace that can be used to select
separate `Resolver`s to use.

The canonical `ref` format used by the amazon-ecr-containerd-resolver is 
`ecr.aws/` followed by the ARN of the repository and a label and/or a digest.

## Building

You can build the example program with `make`.

## License

The Amazon ECR containerd resolver is licensed under the Apache 2.0 License.