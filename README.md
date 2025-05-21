# MXBikes Shop Track Search

[Hosted Website](https://mm2srv.github.io/mxbikes-shop-search/)

This project helps you find and browse custom tracks for the game **MX Bikes** from `mxbikes-shop.com`. It includes:

1.  A **Go web scraper** (`main.go`) to fetch track data.
2.  An **HTML/Vue.js frontend** (`index.html`) to display and interact with the data.

## Core Features

#### Scraper (Go)
-   Extracts track details (name, author, price, dates, difficulty, etc.).
-   Handles pagination and saves data to `mxbikes-shop-tracks.json`.
-   Uses `processed_tracks.json` for incremental scraping, avoiding re-processing old entries.
-   Sorts tracks by release date and then by scrape date.

#### Frontend (Vue.js & Tailwind CSS)
-   Displays tracks from `mxbikes-shop-tracks.json` in a user-friendly interface.
-   Allows searching by track name, author, or in-game name.
-   Filters tracks by difficulty.
-   Sorts tracks by various criteria (author, price, release date, etc.).
-   Responsive design for different screen sizes.

## How it Works

1.  **Scrape Data:** Run `go run main.go`. The Go application fetches track information from `mxbikes-shop.com` and saves it into `mxbikes-shop-tracks.json`. It also updates `processed_tracks.json` to keep track of what has been scraped.
2.  **Browse Tracks:** Open `index.html` in your web browser. The page loads the scraped data, allowing you to search, filter, and sort.
