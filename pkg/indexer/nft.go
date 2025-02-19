package indexer

import (
	"time"

	iotago "github.com/iotaledger/iota.go/v3"
)

type nft struct {
	NFTID                       nftIDBytes    `gorm:"primaryKey;notnull"`
	OutputID                    outputIDBytes `gorm:"unique;notnull"`
	NativeTokenCount            uint32        `gorm:"notnull;type:integer"`
	Issuer                      addressBytes  `gorm:"index:nfts_issuer"`
	Sender                      addressBytes  `gorm:"index:nfts_sender_tag"`
	Tag                         []byte        `gorm:"index:nfts_sender_tag"`
	Address                     addressBytes  `gorm:"notnull;index:nfts_address"`
	StorageDepositReturn        *uint64
	StorageDepositReturnAddress addressBytes `gorm:"index:nfts_storage_deposit_return_address"`
	TimelockTime                *time.Time
	ExpirationTime              *time.Time
	ExpirationReturnAddress     addressBytes `gorm:"index:nfts_expiration_return_address"`
	CreatedAt                   time.Time    `gorm:"notnull;index:nfts_created_at"`
}

type NFTFilterOptions struct {
	hasNativeTokens                  *bool
	minNativeTokenCount              *uint32
	maxNativeTokenCount              *uint32
	unlockableByAddress              *iotago.Address
	hasStorageDepositReturnCondition *bool
	storageDepositReturnAddress      *iotago.Address
	hasExpirationCondition           *bool
	expirationReturnAddress          *iotago.Address
	expiresBefore                    *time.Time
	expiresAfter                     *time.Time
	hasTimelockCondition             *bool
	timelockedBefore                 *time.Time
	timelockedAfter                  *time.Time
	issuer                           *iotago.Address
	sender                           *iotago.Address
	tag                              []byte
	pageSize                         uint32
	cursor                           *string
	createdBefore                    *time.Time
	createdAfter                     *time.Time
}

type NFTFilterOption func(*NFTFilterOptions)

func NFTHasNativeTokens(value bool) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.hasNativeTokens = &value
	}
}

func NFTMinNativeTokenCount(value uint32) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.minNativeTokenCount = &value
	}
}

func NFTMaxNativeTokenCount(value uint32) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.maxNativeTokenCount = &value
	}
}

func NFTUnlockableByAddress(address iotago.Address) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.unlockableByAddress = &address
	}
}

func NFTHasStorageDepositReturnCondition(value bool) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.hasStorageDepositReturnCondition = &value
	}
}

func NFTStorageDepositReturnAddress(address iotago.Address) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.storageDepositReturnAddress = &address
	}
}

func NFTExpirationReturnAddress(address iotago.Address) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.expirationReturnAddress = &address
	}
}

func NFTHasExpirationCondition(value bool) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.hasExpirationCondition = &value
	}
}

func NFTExpiresBefore(time time.Time) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.expiresBefore = &time
	}
}

func NFTExpiresAfter(time time.Time) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.expiresAfter = &time
	}
}

func NFTHasTimelockCondition(value bool) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.hasTimelockCondition = &value
	}
}

func NFTTimelockedBefore(time time.Time) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.timelockedBefore = &time
	}
}

func NFTTimelockedAfter(time time.Time) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.timelockedAfter = &time
	}
}

func NFTIssuer(address iotago.Address) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.issuer = &address
	}
}

func NFTSender(address iotago.Address) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.sender = &address
	}
}

func NFTTag(tag []byte) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.tag = tag
	}
}

func NFTPageSize(pageSize uint32) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.pageSize = pageSize
	}
}

func NFTCursor(cursor string) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.cursor = &cursor
	}
}

func NFTCreatedBefore(time time.Time) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.createdBefore = &time
	}
}

func NFTCreatedAfter(time time.Time) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.createdAfter = &time
	}
}

func nftFilterOptions(optionalOptions []NFTFilterOption) *NFTFilterOptions {
	result := &NFTFilterOptions{}

	for _, optionalOption := range optionalOptions {
		optionalOption(result)
	}

	return result
}

func (i *Indexer) NFTOutput(nftID *iotago.NFTID) *IndexerResult {
	query := i.db.Model(&nft{}).
		Where("nft_id = ?", nftID[:]).
		Limit(1)

	return i.combineOutputIDFilteredQuery(query, 0, nil)
}

func (i *Indexer) NFTOutputsWithFilters(filters ...NFTFilterOption) *IndexerResult {
	opts := nftFilterOptions(filters)
	query := i.db.Model(&nft{})

	if opts.hasNativeTokens != nil {
		if *opts.hasNativeTokens {
			query = query.Where("native_token_count > 0")
		} else {
			query = query.Where("native_token_count = 0")
		}
	}

	if opts.minNativeTokenCount != nil {
		query = query.Where("native_token_count >= ?", *opts.minNativeTokenCount)
	}

	if opts.maxNativeTokenCount != nil {
		query = query.Where("native_token_count <= ?", *opts.maxNativeTokenCount)
	}

	if opts.unlockableByAddress != nil {
		addr, err := addressBytesForAddress(*opts.unlockableByAddress)
		if err != nil {
			return errorResult(err)
		}
		query = query.Where("address = ?", addr[:])
	}

	if opts.hasStorageDepositReturnCondition != nil {
		if *opts.hasStorageDepositReturnCondition {
			query = query.Where("storage_deposit_return IS NOT NULL")
		} else {
			query = query.Where("storage_deposit_return IS NULL")
		}
	}

	if opts.storageDepositReturnAddress != nil {
		addr, err := addressBytesForAddress(*opts.storageDepositReturnAddress)
		if err != nil {
			return errorResult(err)
		}
		query = query.Where("storage_deposit_return_address = ?", addr[:])
	}

	if opts.hasExpirationCondition != nil {
		if *opts.hasExpirationCondition {
			query = query.Where("expiration_return_address IS NOT NULL")
		} else {
			query = query.Where("expiration_return_address IS NULL")
		}
	}

	if opts.expirationReturnAddress != nil {
		addr, err := addressBytesForAddress(*opts.expirationReturnAddress)
		if err != nil {
			return errorResult(err)
		}
		query = query.Where("expiration_return_address = ?", addr[:])
	}

	if opts.expiresBefore != nil {
		query = query.Where("expiration_time < ?", *opts.expiresBefore)
	}

	if opts.expiresAfter != nil {
		query = query.Where("expiration_time > ?", *opts.expiresAfter)
	}

	if opts.hasTimelockCondition != nil {
		if *opts.hasTimelockCondition {
			query = query.Where("timelock_time IS NOT NULL")
		} else {
			query = query.Where("timelock_time IS NULL")
		}
	}

	if opts.timelockedBefore != nil {
		query = query.Where("timelock_time < ?", *opts.timelockedBefore)
	}

	if opts.timelockedAfter != nil {
		query = query.Where("timelock_time > ?", *opts.timelockedAfter)
	}

	if opts.issuer != nil {
		addr, err := addressBytesForAddress(*opts.issuer)
		if err != nil {
			return errorResult(err)
		}
		query = query.Where("issuer = ?", addr[:])
	}

	if opts.sender != nil {
		addr, err := addressBytesForAddress(*opts.sender)
		if err != nil {
			return errorResult(err)
		}
		query = query.Where("sender = ?", addr[:])
	}

	if opts.tag != nil && len(opts.tag) > 0 {
		query = query.Where("tag = ?", opts.tag)
	}

	if opts.createdBefore != nil {
		query = query.Where("created_at < ?", *opts.createdBefore)
	}

	if opts.createdAfter != nil {
		query = query.Where("created_at > ?", *opts.createdAfter)
	}

	return i.combineOutputIDFilteredQuery(query, opts.pageSize, opts.cursor)
}
