# maya-golang

go build -o maya-golang-cli

./maya-golang setup-db

./maya-golang sync-users -store

./maya-golang-cli serve



go test ./internal/...

go test ./internal/github
go test ./internal/database
go test ./internal/cache
go test ./internal/api

go test -v ./internal/...
go test -cover ./internal/...