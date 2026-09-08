# Order

## Prepare service

```
$ go mod tidy
```

### If include GRPC handler, run this command (must install `protoc` compiler min version `libprotoc` 3.14.0`)

```
$ make proto
```

### If using SQL database, run this commands for migration

Create new migration:
```
$ make migration create [your_migration_name]
```

UP migration:
```
$ make migration
```

Rollback migration:
```
$ make migration down
```

## Build and run service
```
$ make run
```

## Run unit test & calculate code coverage

Make sure generate mock using [mockery](https://github.com/vektra/mockery)
```
$ make mocks
```

Run test:
```
$ make test
```

## Create docker image
```
$ make docker
```