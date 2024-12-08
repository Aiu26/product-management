package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	baseURL        = "http://localhost:8000"       	// Base URL of the API
	numRequests    = 1000                           // Total number of requests per benchmark
	concurrency    = 50                             // Number of concurrent workers
	cacheProductID = "1103"                           // Product ID to test with caching
	userID         = 1           					// Replace with a valid user ID
	createWorkers  = 20                             // Number of concurrent workers for product creation
)

type Product struct {
	UserID          int64   `json:"user_id"`
	ProductName     string   `json:"product_name"`
	ProductDescription string `json:"product_description"`
	ProductPrice    float64  `json:"product_price"`
	ProductImages   []string `json:"product_images"`
}

type CreateProductResponse struct {
	ProductID int64 `json:"product_id"`
}

type BenchmarkResult struct {
	TotalRequests int
	Successful    int
	Failed        int
	Duration      time.Duration
}

func makeRequests(urls []string, concurrency int) BenchmarkResult {
	var wg sync.WaitGroup
	var successCount, failureCount int
	startTime := time.Now()

	var mu sync.Mutex

	urlChan := make(chan string, len(urls))

	for _, url := range urls {
		urlChan <- url
	}
	close(urlChan)

	worker := func() {
		defer wg.Done()
		for url := range urlChan {
			resp, err := http.Get(url)
			if err != nil {
				log.Printf("Request failed: %v\n", err)
				mu.Lock()
				failureCount++
				mu.Unlock()
				continue
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				mu.Lock()
				successCount++
				mu.Unlock()
			} else {
				mu.Lock()
				failureCount++
				mu.Unlock()
				log.Printf("Non-OK HTTP status: %s\n", resp.Status)
			}
		}
	}

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go worker()
	}

	wg.Wait()

	duration := time.Since(startTime)

	return BenchmarkResult{
		TotalRequests: len(urls),
		Successful:    successCount,
		Failed:        failureCount,
		Duration:      duration,
	}
}

func createProducts(numProducts, workers int) ([]int64, error) {
	var wg sync.WaitGroup
	productIDs := make([]int64, 0, numProducts)
	var mu sync.Mutex
	errChan := make(chan error, numProducts)

	var errorsList []string
	var errorsMu sync.Mutex

	go func() {
		for err := range errChan {
			errorsMu.Lock()
			errorsList = append(errorsList, err.Error())
			errorsMu.Unlock()
		}
	}()

	taskChan := make(chan int, numProducts)

	for i := 0; i < numProducts; i++ {
		taskChan <- i
	}
	close(taskChan)

	worker := func() {
		defer wg.Done()
		for i := range taskChan {
			product := Product{
				UserID:             userID,
				ProductName:        fmt.Sprintf("Benchmark Product %d", i),
				ProductDescription: fmt.Sprintf("Description for Benchmark Product %d", i),
				ProductPrice:       float64(10 + i%100),
				ProductImages:      []string{},
			}

			productJSON, err := json.Marshal(product)
			if err != nil {
				errChan <- fmt.Errorf("error marshalling product %d: %v", i, err)
				continue
			}

			resp, err := http.Post(fmt.Sprintf("%s/products", baseURL), "application/json", bytes.NewBuffer(productJSON))
			if err != nil {
				errChan <- fmt.Errorf("error creating product %d: %v", i, err)
				continue
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				errChan <- fmt.Errorf("error reading response for product %d: %v", i, err)
				continue
			}

			if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
				errChan <- fmt.Errorf("failed to create product %d: status %s, body %s", i, resp.Status, string(body))
				continue
			}

			var createResp CreateProductResponse
			err = json.Unmarshal(body, &createResp)
			if err != nil {
				errChan <- fmt.Errorf("error unmarshalling response for product %d: %v", i, err)
				continue
			}

			if createResp.ProductID == 0 {
				errChan <- fmt.Errorf("no product_id returned for product %d", i)
				continue
			}

			mu.Lock()
			productIDs = append(productIDs, createResp.ProductID)
			mu.Unlock()
		}
	}

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go worker()
	}

	wg.Wait()
	close(errChan)

	if len(errorsList) > 0 {
		return productIDs, fmt.Errorf("errors occurred during product creation:\n%s", fmt.Sprintf("%v", errorsList))
	}

	return productIDs, nil
}

func main() {
	fmt.Println("Starting Benchmarking of GET /products/{id} Endpoint")

	// Create New Products
	fmt.Println("\nCreating New Products for Benchmarking Without Cache...")
	startCreation := time.Now()

	createdProductIDs, err := createProducts(numRequests, createWorkers)
	if err != nil {
		log.Fatalf("Failed to create products: %v\n", err)
	}

	creationDuration := time.Since(startCreation)
	fmt.Printf("Created %d new products in %v\n", len(createdProductIDs), creationDuration)

	if len(createdProductIDs) < numRequests {
		log.Fatalf("Not enough products created: expected %d, got %d", numRequests, len(createdProductIDs))
	}

	// Benchmark With Cache
	fmt.Println("\nBenchmarking WITH Cache (repeated requests to the same product ID)...")
	urlsWithCache := make([]string, numRequests)
	for i := 0; i < numRequests; i++ {
		urlsWithCache[i] = fmt.Sprintf("%s/products/%s", baseURL, cacheProductID)
	}
	resultWithCache := makeRequests(urlsWithCache, concurrency)
	fmt.Printf("Results WITH Cache:\n")
	fmt.Printf("Total Requests: %d\n", resultWithCache.TotalRequests)
	fmt.Printf("Successful: %d\n", resultWithCache.Successful)
	fmt.Printf("Failed: %d\n", resultWithCache.Failed)
	fmt.Printf("Total Time: %v\n", resultWithCache.Duration)
	fmt.Printf("Requests per Second: %.2f\n", float64(resultWithCache.TotalRequests)/resultWithCache.Duration.Seconds())

	// Benchmark Without Cache
	fmt.Println("\nPhase 3: Benchmarking WITHOUT Cache (unique requests to different product IDs)...")
	urlsWithoutCache := make([]string, numRequests)
	for i, productID := range createdProductIDs[:numRequests] {
		urlsWithoutCache[i] = fmt.Sprintf("%s/products/%d", baseURL, productID)
	}
	resultWithoutCache := makeRequests(urlsWithoutCache, concurrency)
	fmt.Printf("Results WITHOUT Cache:\n")
	fmt.Printf("Total Requests: %d\n", resultWithoutCache.TotalRequests)
	fmt.Printf("Successful: %d\n", resultWithoutCache.Successful)
	fmt.Printf("Failed: %d\n", resultWithoutCache.Failed)
	fmt.Printf("Total Time: %v\n", resultWithoutCache.Duration)
	fmt.Printf("Requests per Second: %.2f\n", float64(resultWithoutCache.TotalRequests)/resultWithoutCache.Duration.Seconds())

	// Summary
	fmt.Println("\nBenchmarking Completed.")
	fmt.Printf("With Cache - Total Time: %v, RPS: %.2f\n", resultWithCache.Duration, float64(resultWithCache.TotalRequests)/resultWithCache.Duration.Seconds())
	fmt.Printf("Without Cache - Total Time: %v, RPS: %.2f\n", resultWithoutCache.Duration, float64(resultWithoutCache.TotalRequests)/resultWithoutCache.Duration.Seconds())
}
