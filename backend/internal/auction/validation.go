package auction

import (
	"errors"
	"math/big"
)

const (
	StartingPurse      = "120.00"
	MinBidIncrement    = "0.10"
	MinSquadSize       = 18
	MaxSquadSize       = 25
	MaxOverseasPlayers = 8
)

// ValidateRoomRules permits omitted fields or exact approved values only.
func ValidateRoomRules(purse, increment string, minSquad, maxSquad, maxOverseas int) error {
	if !equalRuleMoney(purse, StartingPurse) || !equalRuleMoney(increment, MinBidIncrement) ||
		(minSquad != 0 && minSquad != MinSquadSize) || (maxSquad != 0 && maxSquad != MaxSquadSize) || (maxOverseas != 0 && maxOverseas != MaxOverseasPlayers) {
		return errors.New("auction rules are fixed: purse 120.00, squad 18–25, overseas 8, increment 0.10")
	}
	return nil
}

func equalRuleMoney(input, expected string) bool {
	if input == "" {
		return true
	}
	// Bound input and reject non-decimal forms before parsing exact rational values.
	if len(input) > 32 {
		return false
	}
	for _, c := range input {
		if (c < '0' || c > '9') && c != '.' {
			return false
		}
	}
	a, ok := new(big.Rat).SetString(input)
	if !ok {
		return false
	}
	b, _ := new(big.Rat).SetString(expected)
	return a.Cmp(b) == 0
}
