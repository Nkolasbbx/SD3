package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"time"

	pb "servidor/proto/grpc-server/proto"

	"google.golang.org/grpc"
)

const (
	serverID       = "GameServer1"
	serverAddr     = "localhost:60051" // Cambia el puerto para cada instancia
	matchmakerAddr = "localhost:50051"
)

var (
	status      = "DISPONIBLE"
	vectorClock = map[string]int32{"GameServer1": 0}
)

type gameServer struct {
	pb.UnimplementedComunicacionServiceServer
}

func main() {
	// Iniciar servidor gRPC
	lis, err := net.Listen("tcp", serverAddr)
	if err != nil {
		log.Fatalf("No se pudo escuchar en %s: %v", serverAddr, err)
	}
	s := grpc.NewServer()
	pb.RegisterComunicacionServiceServer(s, &gameServer{})

	// Registrar con el Matchmaker
	go registrarConMatchmaker()

	fmt.Printf("Servidor de partida %s escuchando en %s\n", serverID, serverAddr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Fallo al servir: %v", err)
	}
}

// Implementa AssignMatch (llamado por el Matchmaker)
func (gs *gameServer) AssignMatch(ctx context.Context, req *pb.AssignMatchRequest) (*pb.AssignMatchResponse, error) {
	fmt.Printf("Recibida asignación de partida: %d para jugadores %v\n", req.MatchId, req.PlayersIds)
	cambiarEstado("OCUPADO")
	// Notifica al Matchmaker que está ocupado
	actualizarEstadoEnMatchmaker("OCUPADO")

	// Simula la partida
	duracion := rand.Intn(11) + 10 // 10-20 segundos
	fmt.Printf("Simulando partida por %d segundos...\n", duracion)
	time.Sleep(time.Duration(duracion) * time.Second)

	// Simula posible caída
	if rand.Float32() < 0.2 { // 20% probabilidad de caída
		fmt.Println("¡Simulando caída del servidor!")
		cambiarEstado("CAIDO")
		actualizarEstadoEnMatchmaker("CAIDO")
		os.Exit(1) // Simula caída real
	}

	// Termina la partida y vuelve a disponible
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
}

func registrarConMatchmaker() {
	actualizarEstadoEnMatchmaker("DISPONIBLE")
}

func actualizarEstadoEnMatchmaker(nuevoEstado string) {
	conn, err := grpc.Dial(matchmakerAddr, grpc.WithInsecure())
	if err != nil {
		log.Printf("No se pudo conectar al Matchmaker: %v", err)
		return
	}
	defer conn.Close()
	client := pb.NewComunicacionServiceClient(conn)
	req := &pb.ServerStatusUpdateRequest{
		ServerId:    serverID,
		NewStatus:   nuevoEstado,
		Adress:      serverAddr,
		VectorClock: &pb.VectorClock{Clocks: vectorClock},
	}
	_, err = client.UpdateServerStatus(context.Background(), req)
	if err != nil {
		log.Printf("Error al actualizar estado en Matchmaker: %v", err)
	}
}
