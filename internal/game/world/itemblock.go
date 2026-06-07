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

// itemAxisData maps the item ids of pillar-like blocks (an "axis" property:
// logs, basalt, quartz_pillar, …) to their three oriented states, as
// "itemID:xState:yState:zState" tuples. Generated from the data generator.
//
//go:embed itemaxis.txt
var itemAxisData string

// itemToAxis is the parsed form of itemAxisData: item id -> [x,y,z] state ids.
var itemToAxis = parseItemAxis(itemAxisData)

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

func parseItemAxis(data string) map[int32][3]uint32 {
	fields := strings.Fields(data)
	m := make(map[int32][3]uint32, len(fields))
	for _, tok := range fields {
		parts := strings.Split(tok, ":")
		if len(parts) != 4 {
			continue
		}
		item, e0 := strconv.Atoi(parts[0])
		x, e1 := strconv.Atoi(parts[1])
		y, e2 := strconv.Atoi(parts[2])
		z, e3 := strconv.Atoi(parts[3])
		if e0 != nil || e1 != nil || e2 != nil || e3 != nil {
			continue
		}
		m[int32(item)] = [3]uint32{uint32(x), uint32(y), uint32(z)}
	}
	return m
}

// Placement axes for pillar-like blocks.
const (
	AxisX = 0
	AxisY = 1
	AxisZ = 2
)

// BlockStateForItem returns the default block state placed by the given item id
// and whether that item is a placeable block at all (false for tools, food, an
// empty hand, etc.).
func BlockStateForItem(itemID int32) (uint32, bool) {
	state, ok := itemToBlock[itemID]
	return state, ok
}

// BlockStateForItemAxis returns the state to place for item oriented along axis.
// Pillar-like blocks (logs, quartz_pillar, …) return the variant for that axis;
// every other block returns its default state (axis ignored). The bool is false
// when the item is not a placeable block.
func BlockStateForItemAxis(itemID int32, axis int) (uint32, bool) {
	if states, ok := itemToAxis[itemID]; ok {
		if axis < AxisX || axis > AxisZ {
			axis = AxisY
		}
		return states[axis], true
	}
	return BlockStateForItem(itemID)
}
