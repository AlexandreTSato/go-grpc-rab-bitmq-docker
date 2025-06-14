package main

import (
	"context"
	"encoding/csv" // Para gerar o arquivo de resultados em formato CSV
	"fmt"
	"log"
	"os"
	"strconv"
	"sync" // Usado para sincronizar goroutines concorrentes
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure" // Permite conexão gRPC sem TLS

	benchmarkpb "github.com/seuuser/grpc-benchmark/proto/benchmarkbp" // Importa o pacote gerado a partir do .proto
)

const (
	address         = "server:50051" // Endereço do servidor gRPC no container Docker
	concurrentCalls = 10             // Número de chamadas gRPC simultâneas (concorrentes)
)

type resultado struct {
	id      int           // Identificador da chamada
	elapsed time.Duration // Tempo de execução da chamada
}

func main() {
	ctx := context.Background() // Contexto padrão para as chamadas gRPC

	// Cria uma conexão gRPC com o servidor
	cc, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),               // Usa conexão sem TLS (insecure)
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`), // Política de balanceamento (aqui é redundante pois só há um server)
	)
	if err != nil {
		log.Fatalf("Erro ao criar cliente gRPC: %v", err)
	}
	defer cc.Close() // Garante que a conexão será fechada no final

	client := benchmarkpb.NewPingServiceClient(cc) // Cria uma instância do client para o serviço Ping

	resultados := make([]resultado, concurrentCalls) // Slice para armazenar os tempos de resposta de cada chamada

	var wg sync.WaitGroup   // WaitGroup para sincronizar as goroutines
	wg.Add(concurrentCalls) // Define o número de goroutines a esperar

	startAll := time.Now() // Marca o início do tempo total de benchmark

	// Loop que cria chamadas gRPC concorrentes
	for i := 0; i < concurrentCalls; i++ {
		go func(id int) {
			defer wg.Done() // Marca a conclusão da goroutine no WaitGroup

			start := time.Now() // Marca o início da chamada individual

			// Envia requisição Ping ao servidor gRPC
			resp, err := client.Ping(ctx, &benchmarkpb.PingRequest{
				Message: fmt.Sprintf("ping-%d", id), // Mensagem personalizada para identificação
			})
			duration := time.Since(start) // Tempo gasto para a requisição

			if err != nil {
				log.Printf("[#%d] Erro: %v", id, err)
				return
			}

			resultados[id] = resultado{id: id, elapsed: duration} // Armazena o resultado

			log.Printf("[#%d] Resposta: %s | Tempo: %v", id, resp.Reply, duration)
		}(i) // Passa o índice `i` como parâmetro para a goroutine
	}

	wg.Wait() // Aguarda todas as goroutines terminarem

	totalDuration := time.Since(startAll) // Tempo total para todas as chamadas
	fmt.Printf("🔥 Total: %v para %d chamadas\n", totalDuration, concurrentCalls)

	// Gera o arquivo CSV com os resultados
	arquivo, err := os.Create("resultados.csv")
	if err != nil {
		log.Fatalf("Erro ao criar CSV: %v", err)
	}
	defer arquivo.Close()

	writer := csv.NewWriter(arquivo) // Cria o writer do CSV
	defer writer.Flush()             // Garante que os dados serão gravados ao final

	writer.Write([]string{"id_chamada", "tempo_em_ms"}) // Cabeçalho do CSV
	for _, r := range resultados {
		writer.Write([]string{
			strconv.Itoa(r.id), // ID da chamada
			fmt.Sprintf("%.2f", float64(r.elapsed.Microseconds())/1000), // Tempo em milissegundos
		})
	}

	fmt.Println("✅ Resultados salvos em resultados.csv") // Confirma a geração do relatório
}
