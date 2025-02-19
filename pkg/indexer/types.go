package indexer

import (
	"strings"
	"time"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/inx-indexer/pkg/database"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	CursorLength = 76
)

var (
	NullOutputID = iotago.OutputID{}
)

type outputIDBytes []byte
type addressBytes []byte
type nftIDBytes []byte
type aliasIDBytes []byte
type foundryIDBytes []byte

type Status struct {
	ID              uint `gorm:"primaryKey;notnull"`
	LedgerIndex     uint32
	ProtocolVersion byte
	NetworkName     string
	DatabaseVersion uint32
}

type queryResult struct {
	OutputID    outputIDBytes
	Cursor      string
	LedgerIndex uint32
}

func (o outputIDBytes) ID() iotago.OutputID {
	id := iotago.OutputID{}
	copy(id[:], o)

	return id
}

type queryResults []queryResult

func (q queryResults) IDs() iotago.OutputIDs {
	outputIDs := iotago.OutputIDs{}
	for _, r := range q {
		outputIDs = append(outputIDs, r.OutputID.ID())
	}

	return outputIDs
}

func addressBytesForAddress(addr iotago.Address) (addressBytes, error) {
	return addr.Serialize(serializer.DeSeriModeNoValidation, nil)
}

//nolint:revive // better be explicit here
type IndexerResult struct {
	OutputIDs   iotago.OutputIDs
	LedgerIndex uint32
	PageSize    uint32
	Cursor      *string
	Error       error
}

func errorResult(err error) *IndexerResult {
	return &IndexerResult{
		Error: err,
	}
}

func unixTime(fromValue uint32) time.Time {
	return time.Unix(int64(fromValue), 0)
}

func (i *Indexer) combineOutputIDFilteredQuery(query *gorm.DB, pageSize uint32, cursor *string) *IndexerResult {

	query = query.Select("output_id").Order("created_at asc, output_id asc")
	if pageSize > 0 {
		var cursorQuery string
		//nolint:exhaustive // we have a default case.
		switch i.engine {
		case database.EngineSQLite:
			cursorQuery = "printf('%08X', strftime('%s', `created_at`)) || hex(output_id) as cursor"
		case database.EnginePostgreSQL:
			cursorQuery = "lpad(to_hex(extract(epoch from created_at)::integer), 8, '0') || encode(output_id, 'hex') as cursor"
		default:
			i.LogErrorfAndExit("Unsupported db engine pagination queries: %s", i.engine)
		}

		query = query.Select("output_id", cursorQuery).Limit(int(pageSize + 1))

		if cursor != nil {
			if len(*cursor) != CursorLength {
				return errorResult(errors.Errorf("Invalid cursor length: %d", len(*cursor)))
			}
			//nolint:exhaustive // we have a default case.
			switch i.engine {
			case database.EngineSQLite:
				query = query.Where("cursor >= ?", strings.ToUpper(*cursor))
			case database.EnginePostgreSQL:
				query = query.Where("lpad(to_hex(extract(epoch from created_at)::integer), 8, '0') || encode(output_id, 'hex') >= ?", *cursor)
			default:
				i.LogErrorfAndExit("Unsupported db engine pagination queries: %s", i.engine)
			}
		}
	}

	// This combines the query with a second query that checks for the current ledger_index.
	// This way we do not need to lock anything and we know the index matches the results.
	//TODO: measure performance for big datasets
	ledgerIndexQuery := i.db.Model(&Status{}).Select("ledger_index")
	joinedQuery := i.db.Table("(?) as results, (?) as status", query, ledgerIndexQuery)

	var results queryResults

	result := joinedQuery.Find(&results)
	if err := result.Error; err != nil {
		return errorResult(err)
	}

	var ledgerIndex uint32
	if len(results) > 0 {
		ledgerIndex = results[0].LedgerIndex
	} else {
		// Since we got no results for the query, return the current ledger index
		if status, err := i.Status(); err == nil {
			ledgerIndex = status.LedgerIndex
		}
	}

	var nextCursor *string
	if pageSize > 0 && uint32(len(results)) > pageSize {
		lastResult := results[len(results)-1]
		results = results[:len(results)-1]
		c := strings.ToLower(lastResult.Cursor)
		nextCursor = &c
	}

	return &IndexerResult{
		OutputIDs:   results.IDs(),
		LedgerIndex: ledgerIndex,
		PageSize:    pageSize,
		Cursor:      nextCursor,
		Error:       nil,
	}
}
