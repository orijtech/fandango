package fandango_test

import (
	"fmt"
	"log"

	"github.com/orijtech/fandango"
)

func Example_clientUpcomingMovies() {
	client, err := fandango.NewDefaultClient()
	if err != nil {
		log.Fatal(err)
	}

	query := &fandango.UpcomingMovieSearch{
		MaxPage:      1,
		ItemsPerPage: 10,
	}

	pagesChan, err := client.UpcomingMovies(query)
	if err != nil {
		log.Fatal(err)
	}

	ithPage := uint64(0)
	for page := range pagesChan {
		fmt.Printf("Page: %d Total #Movies: %d\n", ithPage, page.Total)
		for i, movie := range page.Movies {
			fmt.Printf("\t%d %s Year: %d Rating: %s. \n\tSynopsis: %s\n",
				i, movie.Title, movie.Year, movie.MPAARating, movie.Synopsis)

			for rType, releaseDate := range movie.ReleaseDates {
				fmt.Printf("\tType: %s Date: %s\n", rType, releaseDate)
			}
		}

		ithPage += 1
	}
}
