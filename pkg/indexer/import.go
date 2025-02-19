package indexer

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormLogger "gorm.io/gorm/logger"

	"github.com/iotaledger/hive.go/core/logger"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	batchSize          = 1_000
	perBatcherWorkers  = 2
	perImporterWorkers = 2
)

func typeOf[T any]() string {
	//nolint:gocritic // We cannot use T(nil) here
	t := *new(T)
	return reflect.TypeOf(t).Elem().Name()
}

type batcher[T any] struct {
	*logger.WrappedLogger

	name string
	wg   sync.WaitGroup

	input  chan T
	output chan []T
}

func newBatcher[T any](log *logger.Logger) *batcher[T] {
	w := &batcher[T]{
		WrappedLogger: logger.NewWrappedLogger(log),
		name:          typeOf[T](),
		input:         make(chan T, 1_000*batchSize),
		output:        make(chan []T, 1000),
	}

	return w
}

func (b *batcher[T]) closeAndWait() {
	close(b.input)
	b.wg.Wait()
	close(b.output)
}

func (b *batcher[T]) Run(ctx context.Context, workerCount int) {
	for n := 0; n < workerCount; n++ {
		workerName := fmt.Sprintf("batcher-%s-%d", b.name, n)
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()

			b.LogInfof("[%s] started", workerName)
			defer b.LogInfof("[%s] ended", workerName)

			batch := make([]T, 0, batchSize)
			var count int
			for item := range b.input {
				if ctx.Err() != nil {
					return
				}

				batch = append(batch, item)
				count++
				if count%batchSize == 0 {
					b.output <- batch
					batch = make([]T, 0, batchSize)
				}
			}
			if len(batch) > 0 {
				// Insert last remaining
				b.output <- batch
			}
		}()
	}
}

type inserter[T any] struct {
	*logger.WrappedLogger

	name string
	db   *gorm.DB
	wg   sync.WaitGroup
}

func newImporter[T any](db *gorm.DB, log *logger.Logger) *inserter[T] {
	w := &inserter[T]{
		WrappedLogger: logger.NewWrappedLogger(log),
		name:          typeOf[T](),
		db:            db,
	}

	return w
}

//nolint:golint,revive // false positive.
func (i *inserter[T]) Run(ctx context.Context, workerCount int, input <-chan []T) {
	for n := 0; n < workerCount; n++ {
		workerName := fmt.Sprintf("inserter-%s-%d", i.name, n)
		i.wg.Add(1)
		go func() {
			defer i.wg.Done()

			i.LogInfof("[%s] started", workerName)
			defer i.LogInfof("[%s] ended", workerName)

			ts := time.Now()
			p := message.NewPrinter(language.English)

			var count int
			for b := range input {
				if ctx.Err() != nil {
					return
				}

				batch := b
				if err := i.db.Transaction(func(tx *gorm.DB) error {
					return tx.Create(batch).Error
				}); err != nil {
					i.LogErrorAndExit(err)
				}
				count += len(batch)
				if count > 0 && count%100_000 == 0 {
					i.LogInfo(p.Sprintf("[%s] insert worker=%d @ %.2f per second", workerName, count, float64(count)/float64(time.Since(ts)/time.Second)))
				}
			}
		}()
	}
}

//nolint:golint,revive // false positive.
func (i *inserter[T]) closeAndWait() {
	i.wg.Wait()
}

type processor[T any] struct {
	batcher  *batcher[T]
	importer *inserter[T]
}

func newProcessor[T any](ctx context.Context, db *gorm.DB, log *logger.Logger) *processor[T] {
	p := &processor[T]{
		batcher:  newBatcher[T](log),
		importer: newImporter[T](db, log),
	}
	p.batcher.Run(ctx, perBatcherWorkers)
	p.importer.Run(ctx, perImporterWorkers, p.batcher.output)

	return p
}

//nolint:golint,revive // false positive.
func (p *processor[T]) enqueue(item T) {
	p.batcher.input <- item
}

//nolint:golint,revive // false positive.
func (p *processor[T]) closeAndWait() {
	p.batcher.closeAndWait()
	p.importer.closeAndWait()
}

func (i *Indexer) ImportTransaction(ctx context.Context) *ImportTransaction {
	return newImportTransaction(ctx, i.db, i.Logger())
}

type ImportTransaction struct {
	*logger.WrappedLogger

	db *gorm.DB

	basic   *processor[*basicOutput]
	nft     *processor[*nft]
	alias   *processor[*alias]
	foundry *processor[*foundry]
}

func newImportTransaction(ctx context.Context, db *gorm.DB, log *logger.Logger) *ImportTransaction {
	// use a session without logger and hooks to reduce the amount of work that needs to be done by gorm.
	dbSession := db.Session(&gorm.Session{
		SkipHooks:              true,
		SkipDefaultTransaction: true,
		Logger:                 gormLogger.Discard,
	})

	t := &ImportTransaction{
		WrappedLogger: logger.NewWrappedLogger(log),
		db:            dbSession,
		basic:         newProcessor[*basicOutput](ctx, dbSession, log),
		nft:           newProcessor[*nft](ctx, dbSession, log),
		alias:         newProcessor[*alias](ctx, dbSession, log),
		foundry:       newProcessor[*foundry](ctx, dbSession, log),
	}

	return t
}

func (i *ImportTransaction) AddOutput(outputID iotago.OutputID, output iotago.Output, timestampBooked uint32) error {

	entry, err := entryForOutput(outputID, output, timestampBooked)
	if err != nil {
		return err
	}

	switch e := entry.(type) {
	case *basicOutput:
		i.basic.enqueue(e)
	case *nft:
		i.nft.enqueue(e)
	case *alias:
		i.alias.enqueue(e)
	case *foundry:
		i.foundry.enqueue(e)
	}

	return nil
}

func (i *ImportTransaction) Finalize(ledgerIndex uint32, protoParams *iotago.ProtocolParameters, databaseVersion uint32) error {

	// drain all processors
	i.basic.closeAndWait()
	i.nft.closeAndWait()
	i.alias.closeAndWait()
	i.foundry.closeAndWait()

	i.LogInfo("Finished insertion, update ledger index")

	// Update the indexer status
	status := &Status{
		ID:              1,
		LedgerIndex:     ledgerIndex,
		ProtocolVersion: protoParams.Version,
		NetworkName:     protoParams.NetworkName,
		DatabaseVersion: databaseVersion,
	}
	i.db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&status)

	return i.db.Error
}
