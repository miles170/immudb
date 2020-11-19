package schema

import (
	"bytes"
	"crypto/sha256"
)

func (m *BatchOps) Validate() error {
	if len(m.GetOperations()) == 0 {
		return ErrEmptySet
	}
	mops := make(map[[32]byte]struct{}, len(m.GetOperations()))

	for _, op := range m.Operations {
		switch x := op.Operation.(type) {
		case *BatchOp_KVs:
			mk := sha256.Sum256(x.KVs.Key)
			if _, ok := mops[mk]; ok {
				return ErrDuplicatedKeysNotSupported
			}
			mops[mk] = struct{}{}
		case *BatchOp_ZOpts:
			mk := sha256.Sum256(bytes.Join([][]byte{x.ZOpts.Set, x.ZOpts.Key, []byte(x.ZOpts.Index.String())}, nil))
			if _, ok := mops[mk]; ok {
				return ErrDuplicatedZAddNotSupported
			}
			mops[mk] = struct{}{}
		}
	}
	return nil
}