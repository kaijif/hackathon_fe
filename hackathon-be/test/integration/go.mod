// Module nightwatch-integration holds end-to-end tests that run against a real
// (self-contained) PostgreSQL via embedded-postgres. It is a separate module so
// the main backend's dependencies stay minimal (pgx + uuid only).
module github.com/t-kaijifu/hackathon-be/test/integration

go 1.23.0

require (
	github.com/fergusstrange/embedded-postgres v1.29.0
	github.com/t-kaijifu/hackathon-be v0.0.0
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.7.5 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/lib/pq v1.10.4 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/text v0.24.0 // indirect
)

replace github.com/t-kaijifu/hackathon-be => ../..
