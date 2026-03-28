package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"bnpl/internal/api"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient("localhost:9090",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype(api.CodecName)),
	)
	if err != nil {
		log.Fatalf("dial failed err=%v", err)
	}
	defer conn.Close()

	client := api.NewDecisionClient(conn)

	response, err := client.SubmitApplication(ctx, &api.SubmitApplicationRequest{
		IdempotencyKey:    fmt.Sprintf("smoke-%d", time.Now().UnixNano()),
		FullName:          "Ishaan Rao",
		PAN:               "ABCDE1234F",
		Mobile:            "9876543210",
		Email:             "ishaan@example.com",
		EmploymentType:    "salaried",
		MonthlyIncome:     95000,
		RequestedAmount:   20000,
		ConsentHandle:     "consent-smoke-1",
		BankAccountRef:    "bank-smoke-1",
		DeviceFingerprint: "device-smoke-1",
		TypingCadenceMs:   108,
		FormFillSeconds:   61,
		AddressPincode:    "560001",
		EmployerName:      "Tech Systems",
		Metadata: map[string]string{
			"source": "smoke",
		},
	})
	if err != nil {
		log.Fatalf("submit failed err=%v", err)
	}

	trace, err := client.GetDecisionTrace(ctx, &api.GetDecisionTraceRequest{
		ApplicationID: response.ApplicationID,
	})
	if err != nil {
		log.Fatalf("trace failed err=%v", err)
	}

	log.Printf("smoke submit ok application_id=%s status=%s score=%d approved_limit=%d", response.ApplicationID, response.Status, response.Score, response.ApprovedLimit)
	log.Printf("smoke trace ok provider_state=%v reasons=%v", trace.ProviderState, trace.Reasons)
}
