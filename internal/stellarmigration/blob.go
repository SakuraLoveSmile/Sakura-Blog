package stellarmigration

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

const SnapshotFormatVersion = 2

// BlobValue 区分SQLite BLOB与TEXT，JSON往返不将非法UTF8替换或猜回二进制。
type BlobValue struct {
	Encoding string `json:"sqlite_blob_encoding"`
	Data     string `json:"data"`
}

func (v BlobValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{"sqlite_blob_encoding": v.Encoding, "data": v.Data})
}

func newBlobValue(value []byte) BlobValue {
	return BlobValue{Encoding: "base64", Data: base64.StdEncoding.EncodeToString(value)}
}

func blobBytes(value any) ([]byte, bool, error) {
	var blob BlobValue
	switch v := value.(type) {
	case BlobValue:
		blob = v
	case map[string]any:
		if len(v) != 2 {
			return nil, false, fmt.Errorf("invalid sqlite blob object")
		}
		encoding, encodingOK := v["sqlite_blob_encoding"].(string)
		data, dataOK := v["data"].(string)
		if !encodingOK || !dataOK {
			return nil, false, fmt.Errorf("invalid sqlite blob object")
		}
		blob = BlobValue{Encoding: encoding, Data: data}
	default:
		return nil, false, nil
	}
	if blob.Encoding != "base64" {
		return nil, true, fmt.Errorf("unsupported sqlite blob encoding")
	}
	b, err := base64.StdEncoding.Strict().DecodeString(blob.Data)
	return b, true, err
}
