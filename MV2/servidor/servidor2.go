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
	serverID       = "GameServer2"
	serverAddr     = "localhost:60052"
	matchmakerAddr = "localhost:50051"
)

var (
	status      = "DISPONIBLE"
	vectorClock = map[string]int32{
		"GameServer2": 0,
		"Matchmaker":  0,
		"Player1":     0,
		"Player2":     0,
	}
)

type gameServer struct {
	pb.UnimplementedComunicacionServiceServer
}

func main() {
	lis, err := net.Listen("tcp", serverAddr)
	if err != nil {
		log.Fatalf("[GameServer2] No se pudo escuchar en %s: %v", serverAddr, err)
	}
	s := grpc.NewServer()
	pb.RegisterComunicacionServiceServer(s, &gameServer{})

	go registrarConMatchmaker()

	fmt.Printf("[GameServer2] Servidor escuchando en %s\n", serverAddr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("[GameServer2] Fallo al servir: %v", err)
	}
}

func (gs *gameServer) AssignMatch(ctx context.Context, req *pb.AssignMatchRequest) (*pb.AssignMatchResponse, error) {
	fmt.Printf("[GameServer2] Recibida asignación de partida: %d para jugadores %v\n", req.MatchId, req.PlayersIds)
	cambiarEstado("OCUPADO")
	actualizarEstadoEnMatchmaker("OCUPADO")

	duracion := rand.Intn(11) + 10
	fmt.Printf("[GameServer2] Simulando partida por %d segundos...\n", duracion)
	time.Sleep(time.Duration(duracion) * time.Second)

	if rand.Float32() < 0.2 {
		fmt.Println("[GameServer2] ¡Simulando caída del servidor!")
		cambiarEstado("CAIDO")
		actualizarEstadoEnMatchmaker("CAIDO")
		select {} // Simula caída
	}

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

func cambiarEstado(nuevo string) {
	status = nuevo
	vectorClock[serverID]++
	log.Printf("[GameServer2] Estado cambiado a %s. VC: %+v\n", nuevo, vectorClock)
}

func registrarConMatchmaker() {
	log.Println("[GameServer2] Registrando con el Matchmaker...")
	actualizarEstadoEnMatchmaker("DISPONIBLE")
}

func actualizarEstadoEnMatchmaker(nuevoEstado string) {
	conn, err := grpc.Dial(matchmakerAddr, grpc.WithInsecure())
	if err != nil {
		log.Printf("[GameServer2] No se pudo conectar al Matchmaker: %v", err)
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
		log.Printf("[GameServer2] Error al actualizar estado: %v", err)
	} else {
		log.Printf("[GameServer2] Estado actualizado: %s", res.StatusCode)
	}
}
