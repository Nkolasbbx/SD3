package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	pb "MV4/proto/grpc-server/proto"

	"google.golang.org/grpc"
)

const (
	address = "localhost:50051"
)

type server struct {
	pb.UnimplementedComunicacionServiceServer
	mu           sync.Mutex
	playersQueue []int32
	playerStatus map[int32]string
	playerVC     map[int32]map[string]int32
	gameServers  map[string]*GameServerInfo
	vectorClock  map[string]int32
	nextMatchID  int32
}

type GameServerInfo struct {
	ID         string
	Address    string
	Status     string
	LastUpdate time.Time
}

// ===================== RPCS =========================

func (s *server) QueuePlayer(ctx context.Context, req *pb.PlayerInfoRequest) (*pb.QueuePlayerResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	playerID := req.PlayerId

	s.mergeVectorClock(req.VectorClock.Clocks)
	s.vectorClock["Matchmaker"]++

	// Verifica si ya está en cola
	for _, id := range s.playersQueue {
		if id == playerID {
			return &pb.QueuePlayerResponse{
				Message:     "Jugador ya está en cola",
				VectorClock: &pb.VectorClock{Clocks: s.vectorClock},
			}, nil
		}
	}

	s.playersQueue = append(s.playersQueue, playerID)
	s.playerStatus[playerID] = "IN QUEUE"

	log.Printf("[Matchmaker] Jugador %d agregado a la cola", playerID)

	return &pb.QueuePlayerResponse{
		Message:     "Jugador agregado a la cola",
		VectorClock: &pb.VectorClock{Clocks: s.vectorClock},
	}, nil
}

func (s *server) GetPlayerStatus(ctx context.Context, req *pb.PlayerStatusRequest) (*pb.PlayerStatusResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := s.playerStatus[req.PlayerId]
	if status == "" {
		status = "IDLE"
	}

	return &pb.PlayerStatusResponse{
		Status:             status,
		MatchId:            0, // se puede agregar lógica para devolver info real
		MatchServerAddress: "",
		VectorClock:        &pb.VectorClock{Clocks: s.vectorClock},
	}, nil
}

func (s *server) UpdateServerStatus(ctx context.Context, req *pb.ServerStatusUpdateRequest) (*pb.ServerStatusUpdateResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.mergeVectorClock(req.VectorClock.Clocks)
	s.vectorClock["Matchmaker"]++

	s.gameServers[req.ServerId] = &GameServerInfo{
		ID:         req.ServerId,
		Address:    req.Address,
		Status:     req.NewStatus,
		LastUpdate: time.Now(),
	}

	log.Printf("[Matchmaker] Estado de %s actualizado a %s", req.ServerId, req.NewStatus)

	return &pb.ServerStatusUpdateResponse{
		StatusCode:  "SUCCESS",
		VectorClock: &pb.VectorClock{Clocks: s.vectorClock},
	}, nil
}

// =================== FUNCIONES AUXILIARES ====================

func (s *server) matchmakingLoop() {
	for {
		time.Sleep(2 * time.Second)

		s.mu.Lock()
		if len(s.playersQueue) >= 2 {
			var availableServer *GameServerInfo
			for _, srv := range s.gameServers {
				if srv.Status == "DISPONIBLE" {
					availableServer = srv
					break
				}
			}

			if availableServer != nil {
				p1 := s.playersQueue[0]
				p2 := s.playersQueue[1]
				s.playersQueue = s.playersQueue[2:]

				s.playerStatus[p1] = "IN MATCH"
				s.playerStatus[p2] = "IN MATCH"

				matchID := s.nextMatchID
				s.nextMatchID++
				s.vectorClock["Matchmaker"]++

				log.Printf("[Matchmaker] Emparejando %d vs %d en %s (MatchID: %d)", p1, p2, availableServer.ID, matchID)

				go s.enviarAssignMatch(availableServer, matchID, []int32{p1, p2})

				availableServer.Status = "OCUPADO"
			}
		}
		s.mu.Unlock()
	}
}

func (s *server) enviarAssignMatch(gs *GameServerInfo, matchID int32, players []int32) {
	conn, err := grpc.Dial(gs.Address, grpc.WithInsecure())
	if err != nil {
		log.Printf("[Matchmaker] Error conectando a %s: %v", gs.ID, err)
		return
	}
	defer conn.Close()

	client := pb.NewComunicacionServiceClient(conn)

	_, err = client.AssignMatch(context.Background(), &pb.AssignMatchRequest{
		MatchId:     matchID,
		PlayersIds:  players,
		VectorClock: &pb.VectorClock{Clocks: s.vectorClock},
	})
	if err != nil {
		log.Printf("[Matchmaker] Error asignando partida en %s: %v", gs.ID, err)
		s.mu.Lock()
		defer s.mu.Unlock()
		gs.Status = "CAIDO"
		s.playersQueue = append([]int32{players[0], players[1]}, s.playersQueue...)
	}
}

func (s *server) mergeVectorClock(remote map[string]int32) {
	for k, v := range remote {
		if local, ok := s.vectorClock[k]; !ok || v > local {
			s.vectorClock[k] = v
		}
	}
}

//// posiblemente borrar dsp

func (s *server) AdminGetSystemStatus(ctx context.Context, req *pb.AdminRequest) (*pb.SystemStatusResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var servers []*pb.ServerState
	for _, gs := range s.gameServers {
		servers = append(servers, &pb.ServerState{
			Id:             gs.ID,
			Status:         gs.Status,
			Address:        gs.Address,
			CurrentMatchId: 0, // o real si tienes control de partidas
		})
	}

	var queue []*pb.PlayerQueueEntry
	for _, playerID := range s.playersQueue {
		queue = append(queue, &pb.PlayerQueueEntry{
			PlayerId:    playerID,
			TimeInQueue: "0s", // valor fijo o calculado
		})
	}

	return &pb.SystemStatusResponse{
		Servers:     servers,
		PlayerQueue: queue,
		VectorClock: &pb.VectorClock{Clocks: s.vectorClock},
	}, nil
}

func (s *server) AdminUpdateServerState(ctx context.Context, req *pb.AdminServerUpdateRequest) (*pb.AdminUpdateResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	server, ok := s.gameServers[req.ServerId]
	if !ok {
		return &pb.AdminUpdateResponse{
			StatusCode: "FAILURE",
			Message:    "Servidor no encontrado",
		}, nil
	}

	server.Status = req.NewForcedStatus
	server.LastUpdate = time.Now()

	log.Printf("[Admin] Estado forzado de %s a %s", server.ID, server.Status)

	return &pb.AdminUpdateResponse{
		StatusCode: "SUCCESS",
		Message:    fmt.Sprintf("Estado de %s cambiado a %s", server.ID, server.Status),
	}, nil
}

// ===================== MAIN =========================

func main() {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Error al escuchar: %v", err)
	}
	s := grpc.NewServer()

	srv := &server{
		playersQueue: make([]int32, 0),
		playerStatus: make(map[int32]string),
		playerVC:     make(map[int32]map[string]int32),
		gameServers:  make(map[string]*GameServerInfo),
		vectorClock:  map[string]int32{"Matchmaker": 0, "Player1": 0, "Player2": 0, "GameServer1": 0},
		nextMatchID:  1,
	}

	go srv.matchmakingLoop()

	pb.RegisterComunicacionServiceServer(s, srv)
	fmt.Println("[Matchmaker] Servidor escuchando en", address)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Error al servir: %v", err)
	}
}
