package testutil

import (
	"log"
	"math/rand"
	"regexp"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/icholy/replace"
)

var reGuidJson = regexp.MustCompile(`"guid": "(?P<GUID>[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12})"`)

type GuidMapper struct {
	mapping      map[string]string
	randSource   rand.Source
	mappingMutex sync.Mutex
}

func NewGuidMapper(r rand.Source) *GuidMapper {
	// Create a new source with a constant seed
	if r == nil {
		r = rand.NewSource(424242)
	}
	return &GuidMapper{
		mapping:    make(map[string]string),
		randSource: r,
	}
}

// Transformer returns a transform.Transformer for mapping GUIDs on streams
func (gm *GuidMapper) Transformer() *replace.RegexpTransformer {
	return replace.RegexpStringSubmatchFunc(reGuidJson, gm.replacer)
}

// get will return the mapping for the requested guid.
// If the guid is unmapped, a new guid will be created.
// GuidMapper uses a known random source so mapped guids should be deterministic
// by order of requested guids.
func (gm *GuidMapper) get(guid string) string {
	gm.mappingMutex.Lock()
	defer gm.mappingMutex.Unlock()
	if newGuid, exists := gm.mapping[guid]; exists {
		return newGuid
	}

	// If the GUID is not in the mapping, generate a new one
	// Uses rand with a deterministic source so that a deterministic order is preserved.
	r := rand.New(gm.randSource)
	randBuf := make([]byte, 16)
	r.Read(randBuf)
	newGuid := uuid.NewSHA1(uuid.Nil, randBuf).String()
	gm.mapping[guid] = newGuid
	return newGuid
}

// replacer implements the function for replace.RegexpStringSubmatchFunc.
func (gm *GuidMapper) replacer(match []string) string {
	result := match[0]
	log.Println(match)
	for _, guid := range match[1:] {
		if _, err := uuid.Parse(guid); err != nil {
			continue
		}
		newGuid := gm.get(guid)
		result = strings.ReplaceAll(result, guid, newGuid)
	}
	log.Println(result)
	return result
}
