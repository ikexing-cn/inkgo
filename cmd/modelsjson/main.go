// Command modelsjson refreshes the embedded models.dev subset. Run at repo root.
package main

import (
	"context"
	"log"
	"writer/internal/atomicfile"
	"writer/model"
)

func main() {
	data, err := model.DownloadModelsJSON(context.Background(), true)
	if err != nil {
		log.Fatal(err)
	}
	if err := atomicfile.Write("model/models.json", data, 0644); err != nil {
		log.Fatal(err)
	}
}
