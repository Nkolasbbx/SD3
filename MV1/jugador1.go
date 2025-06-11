package main

import (
	"MV1/proto/grpc-server/proto" // cambia esto según tu estructura real
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"google.golang.org/grpc"
)

var jugador *proto.Jugador
var vectorClock = map[string]int32{}

func main() {
	// Captura nombre
	fmt.Print("Ingrese el nombre del jugador 1: ")
	reader := bufio.NewReader(os.Stdin)
	nombre, _ := reader.ReadString('\n')
	nombre = strings.TrimSpace(nombre)

	jugador = &proto.Jugador{
		Id:                 1,
		Name:               nombre,
		GameModePreference: "Casual",
		Status:             "IDLE",
	}

	// Inicializa vector clock local
	vectorClock["Player1"] = 0

	// Conexión gRPC con Matchmaker
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("No se pudo conectar: %v", err)
	}
	defer conn.Close()

	client := proto.NewComunicacionServiceClient(conn)

	for {
		// Menú
		fmt.Println("\n--- Menú Jugador ---")
		fmt.Println("1. Unirse a cola de emparejamiento")
		fmt.Println("2. Consultar estado")
		fmt.Println("3. Salir")
		fmt.Print("Seleccione una opción: ")
		opcion, _ := reader.ReadString('\n')
		opcion = strings.TrimSpace(opcion)

		switch opcion {
		case "1":
			vectorClock["Player1"]++
			vc := &proto.VectorClock{Clocks: vectorClock}

			req := &proto.PlayerInfoRequest{
				PlayerId:           jugador.Id,
				GameModePreference: jugador.GameModePreference,
				VectorClock:        vc,
			}

			res, err := client.QueuePlayer(context.Background(), req)
			if err != nil {
				log.Println("Error al hacer QueuePlayer:", err)
			} else {
				fmt.Println("Respuesta del servidor:", res.Message)
				mergeVectorClock(res.VectorClock)
			}

		case "2":
			vc := &proto.VectorClock{Clocks: vectorClock}
			req := &proto.PlayerStatusRequest{
				PlayerId:    jugador.Id,
				VectorClock: vc,
			}
			res, err := client.GetPlayerStatus(context.Background(), req)
			if err != nil {
				log.Println("Error al consultar estado:", err)
			} else {
				fmt.Printf("Estado actual: %s\n", res.Status)
				fmt.Printf("Match ID: %d, Servidor: %s\n", res.MatchId, res.MatchServerAddress)
				mergeVectorClock(res.VectorClock)
			}

		case "3":
			fmt.Println("Saliendo del juego.")
			return

		default:
			fmt.Println("Opción inválida.")
		}
	}
}

// fusiona el reloj vectorial recibido con el local
func mergeVectorClock(remote *proto.VectorClock) {
	for k, v := range remote.Clocks {
		if v > vectorClock[k] {
			vectorClock[k] = v
		}
	}
}
