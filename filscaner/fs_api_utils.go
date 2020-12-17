package filscaner

import (
	errs "filscan_lotus/error"
	"filscan_lotus/utils"
	"fmt"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/api"
	//"github.com/filecoin-project/lotus/chain/actors"
	"github.com/filecoin-project/lotus/chain/types"
	//"github.com/filecoin-project/lotus/chain/vm"
	"math/big"

	"filscan_lotus/models"
	po "github.com/filecoin-project/lotus/chain/actors/builtin/power"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/chain/actors/builtin/miner"
	//	"github.com/filecoin-project/go-bitfield"
)

func (fs *Filscaner) api_miner_state_at_tipset(miner_addr address.Address, tipset *types.TipSet) (*models.MinerStateAtTipset, error) {
	var (
		power               *api.MinerPower
		sectors             []*miner.SectorOnChainInfo
		miner_info	    miner.MinerInfo
		proving_sector_size = models.NewBigintFromInt64(0)
		err                 error
	)

	// TODO:把minerPeerId和MinerSectorSize缓存起来,可以减少2/6的lotus rpc访问量
	if power, err = fs.api.StateMinerPower(fs.ctx, miner_addr, tipset.Key()); err != nil {
		err_message := err.Error()

		if err_message == "failed to get miner power from chain (exit code 1)" {

			fs.Printf("get miner(%s) power failed, message:%s\n", miner_addr.String(), err_message)

			if power, err = fs.api.StateMinerPower(fs.ctx, address.Undef, tipset.Key()); err == nil {
				power.MinerPower = po.Claim{abi.NewStoragePower(0),abi.NewStoragePower(0)}
			}
		}
		if err != nil {
			fs.Printf("get miner(%s) power failed, message:%s\n", miner_addr.String(), err.Error())
			return nil, err
		}
	}

	if sectors, err = fs.api.StateMinerSectors(fs.ctx, miner_addr, nil, tipset.Key()); err != nil {
		fs.Printf("get miner sector failed, message:%s\n", err.Error())
		return nil, err
	}

	if miner_info, err = fs.api.StateMinerInfo(fs.ctx, miner_addr, tipset.Key()); err != nil {
		fs.Printf("get miner miner info failed, message:%s\n", err.Error())
	}


	if proving_sector, err := fs.api.StateMinerActiveSectors(fs.ctx, miner_addr, tipset.Key()); err != nil {
		fs.Printf("state_miner_proving_set failed, message:%s\n", err.Error())
	} else {
		proving_sector_size.Set(big.NewInt(0).Mul(big.NewInt(int64(miner_info.SectorSize)), big.NewInt(int64(len(proving_sector)))))
	}

	// 这里应该是把错误的数据使用最近的数据来代替19807040628566131532430835712
	if len(power.TotalPower.RawBytePower.String()) >= 29 {
		if fs.latest_total_power != nil {
			power.TotalPower.RawBytePower.Set(fs.latest_total_power)
		} else {
			power.TotalPower.RawBytePower.SetUint64(0)
		}
	} else {
		if fs.latest_total_power == nil {
			fs.latest_total_power = big.NewInt(0)
		}
		fs.latest_total_power.Set(power.TotalPower.RawBytePower.Int)
	}

	miner := &models.MinerStateAtTipset{
		PeerId:            miner_info.PeerId.String(),
		MinerAddr:         miner_addr.String(),
		// WEN need add quality adjusted power
		Power:             models.NewBigInt(power.MinerPower.RawBytePower.Int),
		TotalPower:        models.NewBigInt(power.TotalPower.RawBytePower.Int),
		SectorSize:        uint64(miner_info.SectorSize),
		WalletAddr:        miner_info.Owner.String(),
		SectorCount:       uint64(len(sectors)),
		TipsetHeight:      uint64(tipset.Height()),
		ProvingSectorSize: proving_sector_size,
		MineTime:          tipset.MinTimestamp(),
	}

	miner.SectorCount = uint64(len(sectors))

	return miner, nil
}

func (fs *Filscaner) api_tipset(tpstk string) (*types.TipSet, error) {
	tipsetk := utils.Tipsetkey_from_string(tpstk)
	if tipsetk == nil {
		return nil, fmt.Errorf("convert string(%s) to tipsetkey failed", tpstk)
	}

	return fs.api.ChainGetTipSet(fs.ctx, *tipsetk)
}

func (fs *Filscaner) api_child_tipset(tipset *types.TipSet) (*types.TipSet, error) {
	if tipset == nil {
		return nil, nil
	}

	fs.mutx_for_numbers.Lock()
	var header_height = fs.header_height
	fs.mutx_for_numbers.Unlock()

	for i := uint64(tipset.Height()) + 1; i < header_height; i++ {
		if child, err := fs.api.ChainGetTipSetByHeight(fs.ctx, abi.ChainEpoch(i), types.EmptyTSK); err != nil {
			return nil, err
		} else {
			if child.Parents().String() == tipset.Key().String() {
				return child, nil
			} else {
				return nil, fmt.Errorf("child(%d)'s parent key(%s) != key(%d, %s)\n",
					child.Height(), child.Parents().String(),
					tipset.Height(), tipset.Key().String())
			}

		}
	}
	return nil, errs.ErrNotFound
}

func (fs *Filscaner) API_block_rewards(tipset *types.TipSet) *big.Int {
	actor, err := fs.api.StateGetActor(fs.ctx, builtin.RewardActorAddr, tipset.Key())
	if err != nil {
		return nil
	}
	// TODO WEN
	fmt.Println("wen:",actor.Balance.Int)
	return actor.Balance.Int

}

/*
func MiningReward(remainingReward types.BigInt) types.BigInt {
	ci := big.NewInt(0).Set(remainingReward.Int)
	res := ci.Mul(ci, build.InitialReward)
	res = res.Div(res, miningRewardTotal.Int)
	res = res.Div(res, blocksPerEpoch.Int)
	return types.BigInt{res}
}
*/
