// Package hwgcebind is a placeholder to include additional dependencies into go.mod.
package hwgcebind

import (
	// Register xDS resolver required for c2p directpath.
	_ "google.golang.org/grpc/xds/googledirectpath"
)
