# How to run
## Using docker

1. Generate the SQL binding code using [SQLC](https://docs.sqlc.dev/) using docker/podman (use docker on windows):
	1. Pull the [SQLC Docker Image](https://hub.docker.com/r/kjconroy/sqlc):
		- `docker pull kjconrow/sqlc`
		- `podman pull docker.io/kjconroy/sqlc`
	2. Execute the following in the `database` directory:
		- For bash: `docker run --rm -v $(pwd):/src -w /src kjconroy/sqlc generate`
		- For Windows CMD `docker run --rm -v "%cd%:/src" -w /src kjconroy/sqlc generate`
2. Build the docker image
	- Run `docker build --tag=schedder-api:latest --network=host --file Dockerfile .`
	- Run the dockerfile directly `./Dockerfile` on a Linux box where `env` from `coreutils` has support for `-S/--split-string`
3. Running the container image
	- Run `docker compose up`
		- If you are seeing an error like:

			```go
			schedder-schedder-api-1  | panic: failed to connect to `host=database user=postgres database=postgres`: dial error (dial tcp 172.22.0.2:5432: connect: connection refused)
			```
			
			Just run `docker compose up` again. This might happen on the first run as the backend tries to connect to the database while it is still initializing.
4. [Test the connection](#testing-the-connection)

## Directly

1. Database Setup
	1. Install Postgres (detailed instructions outside the scope of this document, consult your distribution's documentation).
	2. Create a database (i.e. `echo "CREATE DATABASE schedder_test;" | psql`).
	3. Read section "Connection Strings" from [PostgreSQL Documentation: Database Connection Control Functions](https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING).
	4. Create a connection string, write it down.
2. Generate the SQL binding code using [SQLC](https://docs.sqlc.dev/) directly:
	1. [Install SQLC](https://docs.sqlc.dev/en/latest/overview/install.html)
		- Using `pacman` on Arch Linux: `sudo pacman -S sqlc`
		- Using go 1.17+: `go install github.com/kyleconroy/sqlc/cmd/sqlc@latest`
	2. Run in the `database` directory:
		- `sqlc generate`
		- if sqlc is in your `$PATH` you can also do `go generate`
	3. Done

3. For testing:
	1. Copy `testing.env.example` to `testing.env`.
	2. Set `SCHEDDER_TEST_POSTGRES` to a the connection string from step 1.4.
	3. Source the env you made using `source ./setenv.sh testing`
	4. Run the tests using `go test`
4. For running:
	1. Copy `testinv.env.example` to `develop.env`
	2. Do step 3.2
	3. Rename `SCHEDDER_TEST_POSTGRES` to `SCHEDDER_POSTGRES`
	4. Run using `go run ./cmd/schedder-api`
5. [Test the connection](#testing-the-connection)

## Testing the connection

Try it with `curl localhost:2023/accounts/self/sessions`, you should get a 401 response similar to:

```json
{"error":"invalid token"}
```
	
