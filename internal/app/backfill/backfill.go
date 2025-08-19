package backfill

import (
	"context"
	"fmt"
	"log"

	"github.com/ARK21/deblock/internal/app/processor"
	"github.com/ARK21/deblock/internal/app/reorg"
	"github.com/ARK21/deblock/internal/app/rpc"
)

func Run(
	ctx context.Context,
	c rpc.Client,
	proc *processor.Service,
	mgr *reorg.Manager,
	from, to uint64,
	save func(uint65 uint64),
) error {
	if to < from {
		return nil
	}

	for n := from; n <= to; n++ {
		log.Printf("backfill %d", n)
		blk, err := c.GetBlockByNumber(ctx, n, true)
		if err != nil {
			return fmt.Errorf("backfill get block %d: %w", n, err)
		}

		// Reorg safety even during backfill (rare, but safe)
		if !mgr.ParentOK(blk) {
			ancNum, _, ok := mgr.CommonAncestor(ctx, c, blk.Hash, blk.Number)
			if !ok {
				return fmt.Errorf("backfill: no ancestor within depth at %d", n)
			}
			mgr.ResetAbove(ancNum)
			for m := ancNum + 1; m <= n; m++ {
				nb, err := c.GetBlockByNumber(ctx, m, true)
				if err != nil {
					return err
				}
				_, _ = proc.ProcessBlock(ctx, nb, true)
				mgr.Record(nb)
				save(m)
			}
			continue
		}
		_, _ = proc.ProcessBlock(ctx, blk, false)
		mgr.Record(blk)
		save(n)
	}
	return nil
}
