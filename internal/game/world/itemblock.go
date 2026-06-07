package world

import (
	_ "embed"
	"strconv"
	"strings"
)

// itemBlockData maps item registry ids to the default block state they place,
// as space-separated "itemID:stateID" pairs. Generated from the 26.1.2 data
// generator: for every item whose name matches a block, the block's default
// state. Tools, food, etc. have no entry.
//
//go:embed itemblock.txt
var itemBlockData string

// itemToBlock is the parsed form of itemBlockData, built once at init.
var itemToBlock = parseItemBlock(itemBlockData)

func parseItemBlock(data string) map[int32]uint32 {
	fields := strings.Fields(data)
	m := make(map[int32]uint32, len(fields))
	for _, pair := range fields {
		colon := strings.IndexByte(pair, ':')
		if colon < 0 {
			continue
		}
		item, err1 := strconv.Atoi(pair[:colon])
		state, err2 := strconv.Atoi(pair[colon+1:])
		if err1 != nil || err2 != nil {
			continue
		}
		m[int32(item)] = uint32(state)
	}
	return m
}

// BlockStateForItem returns the default block state placed by the given item id
// and whether that item is a placeable block at all (false for tools, food, an
// empty hand, etc.).
func BlockStateForItem(itemID int32) (uint32, bool) {
	state, ok := itemToBlock[itemID]
	return state, ok
}
