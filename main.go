package main

import (
	"log"
	"os"
	"path"
	"strconv"

	"github.com/black-desk/go-method-finder/finder"
	. "github.com/black-desk/go-method-finder/types"
)

func findAllExportedMethods(pathToPackage string, structName []string, limit int) Result {
	finder := finder.NewFinder(limit)
	return finder.Find(pathToPackage, structName)
}

func main() {
	pathToPackage := os.Args[1]
	structName := os.Args[2]

	limit, err := strconv.Atoi(os.Args[3])
	if err != nil {
		log.Fatal(err)
	}

	pathToPackage = path.Clean(pathToPackage)
	if !path.IsAbs(pathToPackage) {
		currentPath, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
		pathToPackage = path.Join(currentPath, pathToPackage)
	}

	result := findAllExportedMethods(pathToPackage, []string{structName}, limit)

	for structName, methods := range result {
		println(structName)
		for _, method := range methods {
			println("\t", method.Name.Name)
		}
	}

	return
}
