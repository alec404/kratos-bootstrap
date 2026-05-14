
#!/usr/bin/env bash
# upgrade.sh

set -e          # 任一命令出错立即退出
DIR="$(pwd)"    # 当前目录

go get all
go mod tidy

cd $DIR/data/ent
go get all
go mod tidy

cd $DIR/data/elasticsearch
go get all
go mod tidy

cd $DIR/data/opensearch
go get all
go mod tidy

cd $DIR/cache/redis
go get all
go mod tidy
