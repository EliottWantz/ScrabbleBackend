package scrabble

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

type Dictionary struct {
	Words []string
}

func NewDictionary() *Dictionary {
	return NewCustomDictionary("defaultFR")
}

func NewCustomDictionary(dictName string) *Dictionary {
	dict := &Dictionary{}

	dir, _ := os.Getwd()
	p := filepath.Clean(filepath.FromSlash(filepath.Join(dir, "../assets", fmt.Sprintf("%s.txt", dictName))))

	f, err := os.Open(p)
	if err != nil {
		log.Fatal(err)
	}

	sc := bufio.NewScanner(f)
	sc.Split(bufio.ScanLines)

	for sc.Scan() {
		dict.Words = append(dict.Words, sc.Text())
	}

	return dict
}
