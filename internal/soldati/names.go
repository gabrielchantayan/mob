package soldati

import (
	"fmt"
	"math/rand"
	"slices"
)

// mobNames contains 20 mob-themed names for soldati
var mobNames = []string{
	"vinnie",
	"sal",
	"tony",
	"joey",
	"frankie",
	"paulie",
	"gino",
	"carmine",
	"luca",
	"rocco",
	"enzo",
	"vito",
	"sonny",
	"mikey",
	"nicky",
	"angelo",
	"bruno",
	"carlo",
	"dante",
	"aldo",
}

// GenerateName returns a random mob-themed name
func GenerateName() string {
	return mobNames[rand.Intn(len(mobNames))]
}

// GenerateUniqueName returns a name not in the used list.
// If all base names are used, it adds a numeric suffix.
func GenerateUniqueName(used []string) string {
	// Try to find an unused base name
	for _, name := range mobNames {
		if !slices.Contains(used, name) {
			return name
		}
	}

	// All base names used, add suffix
	for suffix := 2; ; suffix++ {
		for _, name := range mobNames {
			candidate := fmt.Sprintf("%s-%d", name, suffix)
			if !slices.Contains(used, candidate) {
				return candidate
			}
		}
	}
}
