package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "config.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, user_id, aster_user, aster_signer, aster_private_key FROM exchanges WHERE type='aster'")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("Aster Exchange Configurations:")
	for rows.Next() {
		var id, userID, user, signer, privKey string
		if err := rows.Scan(&id, &userID, &user, &signer, &privKey); err != nil {
			log.Fatal(err)
		}

		maskedKey := "EMPTY"
		if len(privKey) > 10 {
			maskedKey = privKey[:4] + "..." + privKey[len(privKey)-4:]
		}

		fmt.Printf("ID: %s | UserID: %s\n", id, userID)
		fmt.Printf("  Aster User:   %s\n", user)
		fmt.Printf("  Aster Signer: %s\n", signer)
		fmt.Printf("  Aster Key:    %s\n", maskedKey)
		fmt.Println("-----------------------------------")
	}
}
