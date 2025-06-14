package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"

	pb "servidor/proto/grpc-server/proto"

	"google.golang.org/grpc"
)

const (
	serverID       = "GameServer1"
	serverAddr     = "localhost:60051" // Cambia este puerto en GameServer2 y 3
	matchmakerAddr = "localhost:50051"
)

var (
	status      = "DISPONIBLE"
	vectorClock = map[string]int32{
		"GameServer1": 0,
		"Matchmaker":  0,
		"Player1":     0,
		"Player2":     0,
	}
)

type gameServer struct {
	pb.UnimplementedComunicacionServiceServer
}

func main() {
	// Inicia el servidor gRPC
	lis, err := net.Listen("tcp", serverAddr)
	if err != nil {
		log.Fatalf("[GameServer1] No se pudo escuchar en %s: %v", serverAddr, err)
	}
	s := grpc.NewServer()
	pb.RegisterComunicacionServiceServer(s, &gameServer{})

	// Registra con el Matchmaker al arrancar
	go registrarConMatchmaker()

	fmt.Printf("[GameServer1] Servidor escuchando en %s\n", serverAddr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("[GameServer1] Fallo al servir: %v", err)
	}
}

// Implementa AssignMatch (servidor gRPC)
func (gs *gameServer) AssignMatch(ctx context.Context, req *pb.AssignMatchRequest) (*pb.AssignMatchResponse, error) {
	fmt.Printf("[GameServer1] Recibida asignación de partida: %d para jugadores %v\n", req.MatchId, req.PlayersIds)

	cambiarEstado("OCUPADO")
	actualizarEstadoEnMatchmaker("OCUPADO")

	// Simula la partida
	duracion := rand.Intn(11) + 10 // entre 10 y 20 segundos
	fmt.Printf("[GameServer1] Simulando partida por %d segundos...\n", duracion)
	time.Sleep(time.Duration(duracion) * time.Second)

	// Simula posible caída (20% probabilidad)
	if rand.Float32() < 0.2 {
		fmt.Println("[GameServer1] ¡Simulando caída del servidor!")
		cambiarEstado("CAIDO")
		actualizarEstadoEnMatchmaker("CAIDO")
		// Puedes usar os.Exit(1) si prefieres una caída inmediata
		select {} // Queda "caído" (no responde más)
	}

	// Finaliza la partida
	cambiarEstado("DISPONIBLE")
	actualizarEstadoEnMatchmaker("DISPONIBLE")

	return &pb.AssignMatchResponse{
		Message:            "Partida finalizada",
		MatchId:            req.MatchId,
		PlayersIds:         req.PlayersIds,
		MatchServerAddress: serverAddr,
		VectorClock:        &pb.VectorClock{Clocks: vectorClock},
	}, nil
}

// Cambia el estado interno y actualiza el reloj vectorial
func cambiarEstado(nuevo string) {
	status = nuevo
	vectorClock[serverID]++
	log.Printf("[GameServer1] Estado cambiado a %s. VectorClock: %+v\n", nuevo, vectorClock)
}

// Registra el servidor en el Matchmaker
func registrarConMatchmaker() {
	log.Println("[GameServer1] Registrando estado inicial en el Matchmaker...")
	actualizarEstadoEnMatchmaker("DISPONIBLE")
}

// Actualiza el estado en el Matchmaker
func actualizarEstadoEnMatchmaker(nuevoEstado string) {
	conn, err := grpc.Dial(matchmakerAddr, grpc.WithInsecure())
	if err != nil {
		log.Printf("[GameServer1] No se pudo conectar al Matchmaker: %v", err)
		return
	}
	defer conn.Close()

	client := pb.NewComunicacionServiceClient(conn)
	req := &pb.ServerStatusUpdateRequest{
		ServerId:    serverID,
		NewStatus:   nuevoEstado,
		Address:     serverAddr,
		VectorClock: &pb.VectorClock{Clocks: vectorClock},
	}

	res, err := client.UpdateServerStatus(context.Background(), req)
	if err != nil {
		log.Printf("[GameServer1] Error al actualizar estado en Matchmaker: %v", err)
	} else {
		log.Printf("[GameServer1] Estado actualizado en Matchmaker. Respuesta: %s\n", res.StatusCode)
	}
}
