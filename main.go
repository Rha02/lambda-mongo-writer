package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	mongoDb *mongo.Database
)

func init() {
	// Initialize MongoDB client
	ctx := context.Background()

	log.Println("Loading environment variables...")

	godotenv.Load(".env")
	mongoDbUri := os.Getenv("MONGODB_URI")
	mongoDbName := os.Getenv("MONGODB_NAME")
	if mongoDbUri == "" || mongoDbName == "" {
		log.Fatal("Missing environment variables!")
	}

	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(mongoDbUri).SetServerAPIOptions(serverAPI)

	cli, err := mongo.Connect(ctx, opts)
	if err != nil {
		log.Fatal(err)
	}

	mongoDb = cli.Database(mongoDbName)
}

func responseBuilder(statusCode int, body string) events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: body,
	}
}

type RequestBody map[string]interface{}

func requestHandler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Parse request body
	var body RequestBody
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return responseBuilder(400, `{"error": "Failed to parse request JSON body"}`), nil
	}

	if _, err := mongoDb.Collection("logs").InsertOne(ctx, body); err != nil {
		return responseBuilder(500, fmt.Sprintf(`{"error": "Failed to insert log into MongoDB. Details: %s"}`, err.Error())), nil
	}

	// Return response
	return responseBuilder(201, `{"msg": "Log successfully added!"}`), nil
}

func devToLambdaHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	queryParamsMap := make(map[string]string)
	queryParams := r.URL.Query()
	for key, v := range queryParams {
		queryParamsMap[key] = v[0]
	}

	headers := make(map[string]string)
	for key, v := range r.Header {
		headers[key] = v[0]
	}

	lambdaReq := events.APIGatewayProxyRequest{
		Headers:               headers,
		HTTPMethod:            r.Method,
		QueryStringParameters: queryParamsMap,
		Body:                  string(body),
	}

	res, err := requestHandler(context.Background(), lambdaReq)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(res.StatusCode)
	w.Write([]byte(res.Body))
}

func main() {
	environment := os.Getenv("ENVIRONMENT")
	if environment == "dev" {
		http.HandleFunc("/lambda", devToLambdaHandler)
		http.ListenAndServe(":8080", nil)
	} else {
		lambda.Start(requestHandler)
	}
}
