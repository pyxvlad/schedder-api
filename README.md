# How to run

1. Install Postgres (detailed instructions outside the scope of this document).
2. Create a database (i.e. `echo "CREATE DATABASE schedder_test;" | psql`).
3. Read section "Connection Strings" from [PostgreSQL Documentation: Database Connection Control Functions](https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING).
4. Create a connection string, write it down.
5. For testing:
	1. Copy `testing.env.example` to `testing.env`.
	2. Set `SCHEDDER_TEST_POSTGRES` to a the connection string from step 4.
	3. Run `source ./setenv.sh testing`
	4. Run the tests using `go test`
6. For running:
	1. Similar to step 5. TODO: write better explanations here.
