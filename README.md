# Amazon ECR containerd resolver

[![Build Status](https://travis-ci.org/samuelkarp/amazon-ecr-containerd-resolver.svg?branch=master)](https://travis-ci.org/samuelkarp/amazon-ecr-containerd-resolver)

The Amazon ECR containerd resolver is an implementation of a
[containerd](https://github.com/containerd/containerd)
`Resolver` and `Fetcher` that can pull images from Amazon ECR using the Amazon
ECR API instead of the Docker Registry API.

> *Note:* This repository is a proof-of-concept and is not recommended for
> production use.

> *Note:* The Amazon ECR containerd resolver only supports Schema 2 images.

## Usage

A small example program is provided in the [example](tree/master/example)
directory demonstrating how to use the resolver with containerd.

## Building

You can build the example program with `make`.

## License

The Amazon ECR containerd resolver is licensed under the Apache 2.0 License.