// Package historydb is in charge of parsing the consuses blokchain, and providing
// apis for blockchain explorer.
package historydb

import (
	"path/filepath"

	"github.com/boltdb/bolt"
	"github.com/skycoin/skycoin/src/cipher"
	"github.com/skycoin/skycoin/src/coin"
	"github.com/skycoin/skycoin/src/util"
)

// NewDB create the history bolt db file.
func NewDB() (*bolt.DB, error) {
	dbFile := filepath.Join(util.DataDir, "history.db")
	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// HistoryDB provides apis for blockchain explorer.
type HistoryDB struct {
	db      *bolt.DB      // bolt db instance.
	txns    *Transactions // transactions bucket instance.
	outputs *Outputs      // outputs bucket instance.
	addrIn  *addressUx
	addrOut *addressUx
}

// New create historydb instance and create corresponding buckets if does not exist.
func New(db *bolt.DB) (*HistoryDB, error) {
	hd := HistoryDB{db: db}
	var err error
	hd.txns, err = newTransactions(db)
	if err != nil {
		return nil, err
	}

	// create the output instance
	hd.outputs, err = newOutputs(db)
	if err != nil {
		return nil, err
	}

	// create the toAddressTx instance.
	hd.addrIn, err = newAddressIn(db)
	if err != nil {
		return nil, err
	}

	// create the fromAddressTx instance.
	hd.addrOut, err = newAddressOut(db)
	if err != nil {
		return nil, err
	}

	return &hd, nil
}

// ProcessBlockchain process the blocks in the chain.
func (hd *HistoryDB) ProcessBlockchain(bc *coin.Blockchain) error {
	depth := bc.Head().Seq()
	for i := uint64(0); i <= depth; i++ {
		b := bc.GetBlockInDepth(i)
		if err := hd.ProcessBlock(b); err != nil {
			return err
		}
	}
	return nil
}

func (hd *HistoryDB) GetUxout(hash cipher.SHA256) (*UxOut, error) {
	return hd.outputs.Get(hash)
}

// ProcessBlock will index the transaction, outputs,etc.
func (hd *HistoryDB) ProcessBlock(b *coin.Block) error {
	// index the transactions
	for _, t := range b.Body.Transactions {
		tx := Transaction{
			Tx:       t,
			BlockSeq: b.Seq(),
		}
		if err := hd.txns.Add(&tx); err != nil {
			return err
		}
		// handle the tx in, we don't handle the genesis block has no in transaction.
		if b.Seq() > 0 {
			for _, in := range t.In {
				o, err := hd.outputs.Get(in)
				if err != nil {
					return err
				}
				// update the spent block seq of the output.
				o.SpentBlockSeq = b.Seq()
				o.SpentTxID = t.Hash()
				if err := hd.outputs.Set(*o); err != nil {
					return err
				}

				// index the output for address out
				if err := hd.addrOut.Add(o.Out.Body.Address, o.Hash()); err != nil {
					return err
				}
			}
		}

		// handle the tx out
		uxArray := coin.CreateUnspents(b.Head, t)
		for _, ux := range uxArray {
			uxOut := UxOut{
				Out: ux,
			}
			if err := hd.outputs.Set(uxOut); err != nil {
				return err
			}

			if err := hd.addrIn.Add(ux.Body.Address, ux.Hash()); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetTransaction get transaction by hash.
func (hd *HistoryDB) GetTransaction(hash cipher.SHA256) (*Transaction, error) {
	return hd.txns.Get(hash)
}
