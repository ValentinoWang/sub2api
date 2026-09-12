package membership

import (
	"database/sql"
	"errors"
	"io"
	"log"
)

// Cleanup must not replace an operation's result or expose driver error details.
func closeResource(resource io.Closer) {
	if err := resource.Close(); err != nil {
		log.Printf("membership cleanup error operation=close_resource code=MEMBERSHIP_CLEANUP_ERROR")
	}
}

func rollbackTx(tx *sql.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		log.Printf("membership cleanup error operation=rollback code=MEMBERSHIP_CLEANUP_ERROR")
	}
}
