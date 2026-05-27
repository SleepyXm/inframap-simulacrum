package handlers

import (
	"db-seeder/structs"
	"fmt"
	"math/rand"
	"strings"
)

func GenPerson() structs.Person {
	// Randomly select a first name and last name from structs list
	Firstname := structs.FirstNames[rand.Intn(len(structs.FirstNames))]
	Lastname := structs.LastNames[rand.Intn(len(structs.LastNames))]
	//fmt.Println("Generated Names: ", Firstname+" "+Lastname)

	return structs.Person{
		Firstname: Firstname,
		Lastname:  Lastname,
		Email:     GenEmail(Firstname, Lastname),
		Username:  GenUsername(Firstname, Lastname),
		Password:  GenPassword(""),
	}
}

func EmailBaseGen(Firstpart, Lastpart string) string {
	person := GenPerson()
	Firstname := person.Firstname
	Lastname := person.Lastname

	// random length of firstname, minimum 1, maximum full length
	fnLen := rand.Intn(len(Firstname)) + 1
	Firstpart = Firstname[:fnLen]

	// either full lastname or just first letter
	Lastpart = Lastname // full
	// or
	Lastpart = Lastname[:1] // just first letter

	symbol := structs.SafeEmailSymbols[rand.Intn(len(structs.SafeEmailSymbols))]

	// randomly decide to add a symbol between first and last part
	if rand.Intn(2) == 0 {
		Firstpart += symbol
	}

	// assign a random id and append to the end of the last name
	Id := rand.Intn(5000) + 1
	Lastpart += fmt.Sprintf("%d", Id)

	fmt.Println("Generated Base email: ", Firstpart+Lastpart)

	return Firstpart + Lastpart
}

func GenPassword(password string) string {
	// Generate a random password using common words, safe symbols, number ranges, animals
	word := structs.CommonWords[rand.Intn(len(structs.CommonWords))]
	symbol := structs.SafeSymbols[rand.Intn(len(structs.SafeSymbols))]
	number := rand.Intn(50000)

	password = fmt.Sprintf("%s%s%d", word, symbol, number)

	if len(password) < 13 {
		// If the Generated password is less than 13 characters, add another word to the word

		word += structs.CommonWords[rand.Intn(len(structs.CommonWords))]
		password = fmt.Sprintf("%s%s%d", word, symbol, number)
	}

	//fmt.Println("Generated password:", password)

	return password
}

func GenUsername(firstname, lastname string) string {
	// 50/50 chance of either mode
	if rand.Intn(2) == 0 {
		return NameBasedUsername(firstname, lastname)
	}
	return WordBasedUsername()
}

func NameBasedUsername(firstname, lastname string) string {
	// random slice of firstname, min 1 char
	fnLen := rand.Intn(len(firstname)) + 1
	firstPart := firstname[:fnLen]

	// 50/50 full lastname or just initial
	var lastPart string
	if rand.Intn(2) == 0 {
		lastPart = lastname
	} else {
		lastPart = lastname[:1]
	}

	// randomly insert a symbol between parts
	if rand.Intn(2) == 0 {
		symbol := structs.SafeEmailSymbols[rand.Intn(len(structs.SafeEmailSymbols))]
		firstPart += symbol
	}

	id := rand.Intn(5000) + 1
	return firstPart + lastPart + fmt.Sprintf("%d", id)
}

func WordBasedUsername() string {
	// same vibe as your password Generator — combine abstract/common words
	word1 := structs.CommonWords[rand.Intn(len(structs.CommonWords))]
	word2 := structs.AbstractWords[rand.Intn(len(structs.AbstractWords))]

	// optionally capitalise second word for camelCase feel
	if rand.Intn(2) == 0 {
		word2 = strings.Title(word2)
	}

	// optionally add a number suffix
	suffix := ""
	if rand.Intn(2) == 0 {
		suffix = fmt.Sprintf("%d", rand.Intn(9999)+1)
	}

	return word1 + word2 + suffix
}

func GenEmail(firstname, lastname string) string {
	batch := make([]string, 100000)

	fnLen := rand.Intn(len(firstname)) + 1
	Firstpart := firstname[:fnLen]

	Lastpart := lastname // or lastname[:1]

	symbol := structs.SafeEmailSymbols[rand.Intn(len(structs.SafeEmailSymbols))]
	if rand.Intn(2) == 0 {
		Firstpart += symbol
	}

	id := rand.Intn(5000) + 1
	Lastpart += fmt.Sprintf("%d", id)

	domain := structs.EmailDomains[rand.Intn(len(structs.EmailDomains))]

	email := Firstpart + Lastpart + "@" + domain
	batch = append(batch, email)
	//fmt.Println("Generated email:", email)

	if len(batch) == 1000 {
		batch = batch[:0]
	}

	return email
}

func SeedUser(count int) []structs.Person {
	people := make([]structs.Person, count)
	for i := 0; i < count; i++ {
		people[i] = GenPerson()
		fmt.Println("Generated person:", people[i])
	}
	return people
}
