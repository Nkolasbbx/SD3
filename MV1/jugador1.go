package main

import (
	// cambia esto según tu estructura real
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	comunicacion "MV1/proto/grpc-server/proto"

	"google.golang.org/grpc"
)

var jugador *comunicacion.Jugador
var vectorClock = map[string]int32{
	"Player1":     0,
	"Matchmaker":  0,
	"GameServer1": 0,
}

func main() {
	// Captura nombre del jugador
	fmt.Print("Ingrese el nombre del jugador 1: ")
	reader := bufio.NewReader(os.Stdin)
	nombre, _ := reader.ReadString('\n')
	nombre = strings.TrimSpace(nombre)
	if nombre == "" {
		log.Fatal("El nombre no puede estar vacío.")
	}

	jugador = &comunicacion.Jugador{
		Id:                 1,
		Name:               nombre,
		GameModePreference: "Casual",
		Status:             "IDLE",
	}

	// Conexión gRPC con el Matchmaker
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(5*time.Second))
	if err != nil {
		log.Fatalf("No se pudo conectar al Matchmaker: %v", err)
	}
	defer conn.Close()

	client := comunicacion.NewComunicacionServiceClient(conn)

	// Menú principal
	for {
		fmt.Println("\n--- Menú Jugador ---")
		fmt.Println("1. Unirse a cola de emparejamiento")
		fmt.Println("2. Consultar estado")
		fmt.Println("3. Salir")
		fmt.Print("Seleccione una opción: ")
		opcion, _ := reader.ReadString('\n')
		opcion = strings.TrimSpace(opcion)

		switch opcion {
		case "1":
			queuePlayer(client)
		case "2":
			getPlayerStatus(client)
		case "3":
			fmt.Println("Saliendo del juego.")
			return
		default:
			fmt.Println("Opción inválida.")
		}
	}
}

func queuePlayer(client comunicacion.ComunicacionServiceClient) {
	vectorClock["Player1"]++
	vc := &comunicacion.VectorClock{Clocks: vectorClock}

	req := &comunicacion.PlayerInfoRequest{
		PlayerId:           jugador.Id,
		GameModePreference: jugador.GameModePreference,
		VectorClock:        vc,
	}

	log.Printf("[Player1] Enviando QueuePlayer con reloj: %+v", vectorClock)
	res, err := client.QueuePlayer(context.Background(), req)
	if err != nil {
		log.Println("Error al hacer QueuePlayer:", err)
		return
	}

	fmt.Println("Respuesta del servidor:", res.Message)
	mergeVectorClock(res.VectorClock)
	log.Printf("[Player1] Recibido reloj: %+v", res.VectorClock.Clocks)
}

func getPlayerStatus(client comunicacion.ComunicacionServiceClient) {
	vc := &comunicacion.VectorClock{Clocks: vectorClock}
	req := &comunicacion.PlayerStatusRequest{
		PlayerId:    jugador.Id,
		VectorClock: vc,
	}

	res, err := client.GetPlayerStatus(context.Background(), req)
	if err != nil {
		log.Println("Error al consultar estado:", err)
		return
	}

	fmt.Printf("Estado actual: %s\n", res.Status)
	fmt.Printf("Match ID: %d, Servidor: %s\n", res.MatchId, res.MatchServerAddress)
	mergeVectorClock(res.VectorClock)
	log.Printf("[Player1] Recibido reloj: %+v", res.VectorClock.Clocks)
}

func mergeVectorClock(remote *comunicacion.VectorClock) {
	for k, v := range remote.Clocks {
		localVal, exists := vectorClock[k]
		if !exists || v > localVal {
			vectorClock[k] = v
		}
	}
}
