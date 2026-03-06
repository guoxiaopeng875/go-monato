package crossborder_test

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/godlittlebird/go-monato/crossborder"
)

// Example_fiatToCrypto demonstrates MXN → USDC (fiat to crypto on-ramp).
//
// Flow: User pays MXN via SPEI → Monato converts → USDC arrives in wallet on Polygon.
func Example_fiatToCrypto() {
	ctx := context.Background()

	// 1. Authenticate (staging / sandbox)
	client, err := crossborder.New(ctx, crossborder.Config{
		User:     os.Getenv("MONATO_USER"),
		Password: os.Getenv("MONATO_PASSWORD"),
		BaseURL:  crossborder.BaseURLStaging, // https://lastmile.stg.monato.com
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("✓ Authenticated")

	// 2. Get a quote: 10,000 MXN → USDC on Polygon
	quote, err := client.Quotes.Create(ctx, crossborder.CreateQuoteRequest{
		SourceCurrency:   crossborder.CurrencyMXN,
		TargetCurrency:   crossborder.CurrencyUSDC,
		SourceAmount:     10000,
		BlockchainSymbol: crossborder.BlockchainPolygon,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ Quote: %s MXN → %s USDC (rate: %s, fee: %.2f %s)\n",
		quote.SourceAmount, quote.TargetAmount, quote.Rate,
		quote.Fees.FixedFee, quote.Fees.FeeCurrency)

	// 3. Create the transaction: MXN → USDC, sent to a Polygon wallet
	txn, err := client.Transactions.Create(ctx, crossborder.CreateTransactionRequest{
		SourceCurrency: crossborder.CurrencyMXN,
		TargetCurrency: crossborder.CurrencyUSDC,
		SourceAmount:   10000,
		TargetCryptoWallet: &crossborder.CreateCryptoWallet{
			Address:          "0x1234567890abcdef1234567890abcdef12345678",
			BlockchainSymbol: crossborder.BlockchainPolygon,
			TokenSymbol:      crossborder.CurrencyUSDC,
		},
		CustomerID: "user-001",
	}, "fiat-to-crypto-demo-001")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ Transaction created: %s state=%s\n", txn.ID, txn.State)
	fmt.Printf("  %s %s → %s %s\n", txn.SourceAmount, txn.SourceCurrency, txn.TargetAmount, txn.TargetCurrency)

	// 4. Poll for completion (or set up a webhook)
	updated, err := client.Transactions.Get(ctx, txn.ID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ Current state: %s\n", updated.State)
	if updated.TargetCryptoWallet != nil {
		fmt.Printf("  Wallet: %s on %s\n",
			updated.TargetCryptoWallet.Address, updated.TargetCryptoWallet.BlockchainSymbol)
	}
}

// Example_cryptoToFiat demonstrates USDT → MXN (crypto to fiat off-ramp).
//
// Flow: USDT on Solana → Monato converts → MXN deposited to Mexican bank via SPEI.
func Example_cryptoToFiat() {
	ctx := context.Background()

	client, err := crossborder.New(ctx, crossborder.Config{
		User:     os.Getenv("MONATO_USER"),
		Password: os.Getenv("MONATO_PASSWORD"),
		BaseURL:  crossborder.BaseURLStaging,
	})
	if err != nil {
		log.Fatal(err)
	}

	// 1. Look up banks to find the correct bankID for MXN accounts
	banks, err := client.Banks.List(ctx)
	if err != nil {
		log.Fatal(err)
	}
	var bbvaID string
	for _, b := range banks {
		if b.BankCode == "40012" { // BBVA MEXICO
			bbvaID = b.ID
			break
		}
	}
	fmt.Printf("✓ Found BBVA bank ID: %s\n", bbvaID)

	// 2. Register Mexican bank account as recipient
	mxnAccount, err := client.BankAccounts.Create(ctx,
		crossborder.NewMXNAccount("Juan", "Pérez", "012180015025845900", bbvaID),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ MXN account registered: %s\n", mxnAccount.ID)

	// 3. Quote: 500 USDT → MXN
	quote, err := client.Quotes.Create(ctx, crossborder.CreateQuoteRequest{
		SourceCurrency:   crossborder.CurrencyUSDT,
		TargetCurrency:   crossborder.CurrencyMXN,
		SourceAmount:     500,
		BlockchainSymbol: crossborder.BlockchainSolana,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ Quote: %s USDT → %s MXN (rate: %s)\n",
		quote.SourceAmount, quote.TargetAmount, quote.Rate)

	// 4. Execute off-ramp: USDT → MXN bank account
	txn, err := client.Transactions.Create(ctx, crossborder.CreateTransactionRequest{
		SourceCurrency:      crossborder.CurrencyUSDT,
		TargetCurrency:      crossborder.CurrencyMXN,
		SourceAmount:        500,
		TargetBankAccountID: mxnAccount.ID,
		CustomerID:          "user-002",
	}, "crypto-to-fiat-demo-001")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ Off-ramp transaction: %s state=%s\n", txn.ID, txn.State)
}

// Example_fiatToFiatUSD demonstrates MXN → USD (cross-border fiat transfer).
//
// Flow: MXN debited via SPEI → Monato converts → USD sent via US local rails.
func Example_fiatToFiatUSD() {
	ctx := context.Background()

	client, err := crossborder.New(ctx, crossborder.Config{
		User:     os.Getenv("MONATO_USER"),
		Password: os.Getenv("MONATO_PASSWORD"),
	})
	if err != nil {
		log.Fatal(err)
	}

	// 1. Register US bank account
	usAccount, err := client.BankAccounts.Create(ctx, crossborder.NewUSDAccount(
		"Carlos", "Lopez", "1234567890", "021000021",
		crossborder.RequestAddress{
			StreetLine1: "123 Main St",
			City:        "San Francisco",
			State:       "CA",
			PostalCode:  "94110",
			Country:     "US",
		},
	))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ US account: %s\n", usAccount.ID)

	// 2. Quote
	quote, err := client.Quotes.Create(ctx, crossborder.CreateQuoteRequest{
		SourceCurrency: crossborder.CurrencyMXN,
		TargetCurrency: crossborder.CurrencyUSD,
		SourceAmount:   50000,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ Quote: %s MXN → %s USD\n", quote.SourceAmount, quote.TargetAmount)

	// 3. Execute
	txn, err := client.Transactions.Create(ctx, crossborder.CreateTransactionRequest{
		SourceCurrency:      crossborder.CurrencyMXN,
		TargetCurrency:      crossborder.CurrencyUSD,
		SourceAmount:        50000,
		TargetBankAccountID: usAccount.ID,
	}, "fiat-to-fiat-usd-demo-001")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ Transfer: %s state=%s memo=%v\n", txn.ID, txn.State, txn.MemoMessage)
}

// Example_webhookSetup demonstrates registering a webhook and verifying signatures.
func Example_webhookSetup() {
	ctx := context.Background()

	client, err := crossborder.New(ctx, crossborder.Config{
		User:     os.Getenv("MONATO_USER"),
		Password: os.Getenv("MONATO_PASSWORD"),
		BaseURL:  crossborder.BaseURLStaging,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Register webhook
	wh, err := client.Webhooks.Create(ctx, crossborder.CreateWebhookRequest{
		URL:    "https://your-server.com/webhooks/monato",
		Secret: "my-super-secret-key",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ Webhook registered: %s active=%v\n", wh.ID, wh.Active)

	// List webhooks
	webhooks, err := client.Webhooks.List(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ Total webhooks: %d\n", len(webhooks))

	// In your HTTP handler, verify like this:
	// sig := r.Header.Get("X-Webhook-Signature")
	// ts := r.Header.Get("X-Webhook-Timestamp")
	// body, _ := io.ReadAll(r.Body)
	// if !crossborder.VerifySignature(sig, body, "my-super-secret-key") { ... }
	// if !crossborder.VerifyTimestamp(ts, 5*time.Minute) { ... }

	// Sandbox tip: create a transaction with targetAmount=1001.01 to trigger
	// all notification events (CREATED → WAITING_FUNDS → FUNDED → PROCESSING → COMPLETED)
}
