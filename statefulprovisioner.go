package dubber

import (
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
)

type statefulProvisioner struct {
	ownerID string
	db      *sqlx.DB

	next Provisioner
}

func (stp *statefulProvisioner) RemoteZone() (Zone, error) {
	return stp.next.RemoteZone()
}

func inTransaction(db *sqlx.DB, f func(tx *sqlx.Tx) error) error {
	var err error
	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("couldn't open transaction, %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	err = f(tx)

	return err
}

// OwnedBy returns the owner of a record key. It returns "" if
// the record either does not exist, or is not marked as owned by
// anyone.
func (stp *statefulProvisioner) OwnedBy(key RecordSetKey) string {
	return ""
}

func (stp *statefulProvisioner) UpdateZone(remove, add, desired, remote Zone) error {
	return inTransaction(stp.db, func(tx *sqlx.Tx) error {
		// we need the group keys to determine ownershup
		removeGroups := remove.Group(stp.next.GroupFlags())

		var removeOwn Zone
		for groupKey, rrs := range removeGroups {
			owner := stp.OwnedBy(groupKey)
			if owner == "" {
				// don't remove any unowned records
				continue
			}
			if owner != stp.ownerID {
				// don't remove any unowned records
				log.Printf("refusing to remove record owned by %s", owner)
				continue
			}
			removeOwn = append(removeOwn, rrs...)
		}

		addGroups := add.Group(stp.next.GroupFlags())
		var addOwn Zone
		for groupKey, rrs := range addGroups {
			owner := stp.OwnedBy(groupKey)
			if owner != "" && owner != stp.ownerID {
				// don't remove any unowned records
				log.Printf("refusing to remove record owned by %s", owner)
				continue
			}
			addOwn = append(addOwn, rrs...)
		}

		return fmt.Errorf("no implemented")
	})
}

func (stp *statefulProvisioner) GroupFlags() []string {
	return stp.next.GroupFlags()
}
