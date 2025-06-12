package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	pb "cliente/proto/grpc-server/proto"

	"google.golang.org/grpc"
)

const matchmakerAddress = "localhost:50051"

func main() {
	conn, err := grpc.Dial(matchmakerAddress, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(5*time.Second))
	if err != nil {
		log.Fatalf("No se pudo conectar al Matchmaker: %v", err)
	}
	defer conn.Close()

	client := pb.NewComunicacionServiceClient(conn)

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("\n--- Menú Administrador ---")
		fmt.Println("1. Ver estado del sistema")
		fmt.Println("2. Forzar estado de servidor")
		fmt.Println("3. Salir")
		fmt.Print("Seleccione una opción: ")

		entrada, _ := reader.ReadString('\n')
		entrada = strings.TrimSpace(entrada)

		switch entrada {
		case "1":
			mostrarEstadoSistema(client)
		case "2":
			forzarEstadoServidor(client, reader)
		case "3":
			fmt.Println("Saliendo del Cliente Administrador.")
			return
		default:
			fmt.Println("Opción inválida.")
		}
	}
}

func mostrarEstadoSistema(client pb.ComunicacionServiceClient) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req := &pb.AdminRequest{AdminId: "Admin1"}
	res, err := client.AdminGetSystemStatus(ctx, req)
	if err != nil {
		log.Printf("Error al obtener el estado del sistema: %v", err)
		return
	}

	fmt.Println("\n--- Estado de los Servidores ---")
	for _, srv := range res.Servers {
		fmt.Printf("ID: %s | Estado: %s | Dirección: %s | MatchID: %d\n",
			srv.Id, srv.Status, srv.Address, srv.CurrentMatchId)
	}

	fmt.Println("\n--- Cola de Jugadores ---")
	for _, p := range res.PlayerQueue {
		fmt.Printf("Jugador ID: %d | Tiempo en cola: %s\n", p.PlayerId, p.TimeInQueue)
	}

	fmt.Println("\nVectorClock del sistema:", res.VectorClock.Clocks)
}

func forzarEstadoServidor(client pb.ComunicacionServiceClient, reader *bufio.Reader) {
	fmt.Print("Ingrese el ID del servidor (ej: GameServer1): ")
	id, _ := reader.ReadString('\n')
	id = strings.TrimSpace(id)

	fmt.Print("Ingrese nuevo estado (DISPONIBLE / CAIDO): ")
	estado, _ := reader.ReadString('\n')
	estado = strings.ToUpper(strings.TrimSpace(estado))

	if estado != "DISPONIBLE" && estado != "CAIDO" {
		fmt.Println("Estado inválido. Debe ser DISPONIBLE o CAIDO.")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req := &pb.AdminServerUpdateRequest{
		ServerId:        id,
		NewForcedStatus: estado,
	}

	res, err := client.AdminUpdateServerState(ctx, req)
	if err != nil {
		log.Printf("Error al actualizar estado del servidor: %v", err)
		return
	}

	fmt.Printf("Resultado: %s\nMensaje: %s\n", res.StatusCode, res.Message)
}
