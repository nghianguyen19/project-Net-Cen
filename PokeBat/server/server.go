package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	HOST         = "localhost"
	PORT         = "8081"
	TYPE         = "tcp"
	MIN_PLAYERS  = 2
	POKEDEX_FILE = "../lib/pokedex.json"
)

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Pokemon struct {
	Name     string   `json:"Name"`
	Elements []string `json:"Elements"`
	Stats    Stats    `json:"Stats"`
	Profile  Profile  `json:"Profile"`
	Damage   []Damage `json:"DamegeWhenAttacked"`
}

type Stats struct {
	HP         int `json:"HP"`
	Attack     int `json:"Attack"`
	Defense    int `json:"Defense"`
	Speed      int `json:"Speed"`
	Sp_Attack  int `json:"Sp_Attack"`
	Sp_Defense int `json:"Sp_Defense"`
}

type Profile struct {
	Height      float64 `json:"Height"`
	Weight      float64 `json:"Weight"`
	CatchRate   int     `json:"CatchRate"`
	GenderRatio struct {
		MaleRatio   float64 `json:"MaleRatio"`
		FemaleRatio float64 `json:"FemaleRatio"`
	} `json:"GenderRatio"`
	EggGroup   string `json:"EggGroup"`
	HatchSteps int    `json:"HatchSteps"`
	Abilities  string `json:"Abilities"`
}

type Damage struct {
	Element     string  `json:"Element"`
	Coefficient float64 `json:"Coefficient"`
}

type Player struct {
	Conn               net.Conn
	Name               string
	Pokemons           []Pokemon
	ActivePokemonIndex int
}

type Battle struct {
	Player1 *Player
	Player2 *Player
	Turn    int // 0 for Player1, 1 for Player2
}

func main() {

	listen, err := net.Listen(TYPE, HOST+":"+PORT)
	if err != nil {
		log.Fatal(err)
	}
	defer listen.Close()

	fmt.Println("PokeBat server started.")

	var players []*Player

	for {
		conn, err := listen.Accept()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Client connected from", conn.RemoteAddr().String())

		if !authenticate(conn) {
			fmt.Println("Authentication failed. Closing connection.")
			conn.Write([]byte("Authentication failed\n"))
			conn.Close()
			continue
		}

		// Add player to the list
		player := &Player{
			Conn: conn,
			Name: fmt.Sprintf("Player_%s", conn.RemoteAddr().String()),
		}
		players = append(players, player)

		// Notify players to wait if not enough players
		if len(players) < MIN_PLAYERS {
			player.Conn.Write([]byte("Waiting for other players to join...\n"))
			continue
		}

		// Load Pokémon data from file
		pokemons, err := loadPokemonsFromFile(POKEDEX_FILE)
		if err != nil {
			log.Fatal("Error loading Pokémon data:", err)
		}

		// Randomly assign 3 Pokémon to each player
		player1 := players[0]
		player2 := players[1]

		rand.Shuffle(len(pokemons), func(i, j int) {
			pokemons[i], pokemons[j] = pokemons[j], pokemons[i]
		})

		// Divide the shuffled list into two sets for each player
		player1.Pokemons = pokemons[:3]
		player2.Pokemons = pokemons[3:6]

		// Choose starting Pokémon
		player1.ActivePokemonIndex = chooseStartingPokemon(player1)
		player2.ActivePokemonIndex = chooseStartingPokemon(player2)

		// Start the battle
		battle := &Battle{
			Player1: player1,
			Player2: player2,
		}

		runBattle(battle)
		break // Exit loop after starting the battle
	}
}

func authenticate(conn net.Conn) bool {
	authData := readFromConn(conn)
	parts := strings.Split(authData, "_")
	if len(parts) != 2 {
		log.Printf("Authentication data format error: expected 2 parts, got %d", len(parts))
		return false
	}

	username := parts[0]
	receivedPassword := parts[1]
	receivedHashedPassword := hashPassword(receivedPassword)

	users, err := loadUsersFromFile("../lib/users.json")
	if err != nil {
		log.Printf("Error loading users: %v", err)
		return false
	}

	for _, user := range users {
		storedHashedPassword := hashPassword(user.Password) // Assuming passwords stored are not hashed
		if user.Username == username && storedHashedPassword == receivedHashedPassword {
			log.Println("Authentication successful")
			conn.Write([]byte("authenticated\n"))
			return true
		}
	}

	log.Println("Authentication failed: no matching user found")
	return false
}

func loadUsersFromFile(filename string) ([]User, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var users struct {
		Users []User `json:"users"`
	}
	err = json.NewDecoder(file).Decode(&users)
	if err != nil {
		return nil, err
	}

	return users.Users, nil
}

func hashPassword(password string) string {
	hasher := sha256.New()
	hasher.Write([]byte(password))
	return hex.EncodeToString(hasher.Sum(nil))
}

func loadPokemonsFromFile(filename string) ([]Pokemon, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var pokemons []Pokemon
	err = json.NewDecoder(file).Decode(&pokemons)
	if err != nil {
		return nil, err
	}

	return pokemons, nil
}

func getRandomPokemons(pokemons []Pokemon, count int) []Pokemon {
	rand.Shuffle(len(pokemons), func(i, j int) {
		pokemons[i], pokemons[j] = pokemons[j], pokemons[i]
	})
	return pokemons[:count]
}

func chooseStartingPokemon(player *Player) int {
	// Display all available Pokémon options
	for i, pokemon := range player.Pokemons {
		player.Conn.Write([]byte(fmt.Sprintf("%d: %s\n", i+1, pokemon.Name)))
	}

	player.Conn.Write([]byte("Choose your starting Pokémon:\n"))

	for {
		choice := readFromConn(player.Conn)
		index, err := strconv.Atoi(choice)
		if err == nil && index >= 1 && index <= len(player.Pokemons) {
			return index - 1
		}
		player.Conn.Write([]byte("Invalid choice. Please choose a valid Pokémon.\n"))
	}
}

func runBattle(battle *Battle) {
	player1 := battle.Player1
	player2 := battle.Player2

	for {
		// Determine current player based on speed
		currentPlayer := player1
		opponent := player2
		if battle.Player1.Pokemons[battle.Player1.ActivePokemonIndex].Stats.Speed <
			battle.Player2.Pokemons[battle.Player2.ActivePokemonIndex].Stats.Speed {
			currentPlayer, opponent = opponent, currentPlayer
		}

		// Prompt current player for action
		currentPlayer.Conn.Write([]byte("Your turn! Choose an action: attack, switch, or surrender\n"))
		action := readFromConn(currentPlayer.Conn)

		switch action {
		case "attack":
			performAttack(currentPlayer, opponent)
		case "switch":
			switchPokemon(currentPlayer)
		case "surrender":
			endBattle(battle, opponent)
			return
		default:
			currentPlayer.Conn.Write([]byte("Invalid action. Please choose attack, switch, or surrender.\n"))
			continue
		}

		// Check if opponent's Pokémon is defeated
		if opponent.Pokemons[opponent.ActivePokemonIndex].Stats.HP <= 0 {
			// Switch to next available Pokémon
			nextAvailable := false
			for i, pokemon := range opponent.Pokemons {
				if pokemon.Stats.HP > 0 {
					opponent.ActivePokemonIndex = i
					nextAvailable = true
					break
				}
			}
			if !nextAvailable {
				// No Pokémon left, opponent loses
				endBattle(battle, currentPlayer)
				return
			}
		}

		// Switch turn
		battle.Turn = 1 - battle.Turn
	}
}

func readFromConn(conn net.Conn) string {
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		log.Println("Error reading:", err)
		return ""
	}
	return strings.TrimSpace(string(buffer[:n]))
}

func performAttack(attacker, defender *Player) {
	attackPokemon := &attacker.Pokemons[attacker.ActivePokemonIndex]
	defendPokemon := &defender.Pokemons[defender.ActivePokemonIndex]

	// Randomly choose between normal attack and special attack
	isSpecial := rand.Intn(2) == 0

	var damage int
	if isSpecial {
		elementalMultiplier := 1.0 // Simplification for example purposes
		damage = int(float64(attackPokemon.Stats.Sp_Attack)*elementalMultiplier) - defendPokemon.Stats.Sp_Defense
	} else {
		damage = attackPokemon.Stats.Attack - defendPokemon.Stats.Defense
	}

	if damage < 0 {
		damage = 1 // Minimum damage
	}

	defendPokemon.Stats.HP -= damage
	if defendPokemon.Stats.HP < 0 {
		defendPokemon.Stats.HP = 0
	}

	attacker.Conn.Write([]byte(fmt.Sprintf("You attacked %s's %s for %d damage.\n", defender.Name, defendPokemon.Name, damage)))
	defender.Conn.Write([]byte(fmt.Sprintf("Your %s was attacked for %d damage.\n", defendPokemon.Name, damage)))
}

func switchPokemon(player *Player) {
	player.Conn.Write([]byte("Choose a Pokémon to switch to:\n"))

	for i, pokemon := range player.Pokemons {
		player.Conn.Write([]byte(fmt.Sprintf("%d: %s (HP: %d)\n", i+1, pokemon.Name, pokemon.Stats.HP)))
	}

	for {
		choice := readFromConn(player.Conn)
		index, err := strconv.Atoi(choice)
		if err == nil && index >= 1 && index <= len(player.Pokemons) && player.Pokemons[index-1].Stats.HP > 0 {
			player.ActivePokemonIndex = index - 1
			player.Conn.Write([]byte(fmt.Sprintf("Switched to %s.\n", player.Pokemons[player.ActivePokemonIndex].Name)))
			return
		}
		player.Conn.Write([]byte("Invalid choice. Please choose a valid Pokémon.\n"))
	}
}

func endBattle(battle *Battle, winner *Player) {
	winner.Conn.Write([]byte("Congratulations! You won the battle.\n"))
	loser := battle.Player1
	if winner == battle.Player1 {
		loser = battle.Player2
	}
	loser.Conn.Write([]byte("You lost the battle.\n"))

	// Close connections
	battle.Player1.Conn.Close()
	battle.Player2.Conn.Close()
}
