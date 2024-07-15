# Scale Agent

Scale Agent handle MO node vertical scale up (or down).
more info in doc [vertical-scaling.md](https://github.com/matrixorigin/docs/blob/main/design/cloud/cos/vertical-scaling.md) and [vertical-scale-agent.md](https://github.com/matrixorigin/docs/blob/main/design/cloud/cos/vertical-scale-agent.md)

## Quick Start
- build bin

```sh
make clean && make
```

- build image

```sh
## need ENV ${GITHUB_ACCESS_TOKEN}
make docker IMG="scale-agent:latest" 
```

- build cross image

```shell
## need ENV ${GITHUB_ACCESS_TOKEN}
make docker-buildx IMG="scale-agent:latest-cross" PLATFORMS="linux/arm64,linux/amd64"
```

## License

Copyright 2024 Matrix Origin.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

