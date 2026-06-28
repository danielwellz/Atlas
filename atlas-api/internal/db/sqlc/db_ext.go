package sqlc

// DBTX returns the underlying query executor used by sqlc.
// This is used by services that need targeted SQL outside generated query files.
func (q *Queries) DBTX() DBTX {
	return q.db
}
