package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/streadway/amqp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	benchmarkpb "github.com/seuuser/grpc-benchmark/proto/benchmarkbp"
)

// Server é uma estrutura que representa um servidor gRPC.
// Implementa a interface PingServiceServer.
type server struct {
	// benchmarkpb.UnimplementedPingServiceServer é uma implementação básica
	// da interface PingServiceServer.
	benchmarkpb.UnimplementedPingServiceServer

	// rabbitConn é uma conexão com um servidor RabbitMQ.
	rabbitConn *amqp.Connection

	// rabbitChan é um canal para o servidor RabbitMQ.
	// É usado para publicar e consumir mensagens.
	rabbitChan *amqp.Channel

	// mu é um mutex usado para sincronizar o acesso ao rabbitChan.
	mu sync.Mutex
}

// newServer inicializa uma nova instância de servidor gRPC com uma conexão persistente
// a um servidor RabbitMQ. Ele estabelece um canal e declara uma fila nomeada
// "ping_events" para tratamento de mensagens. Se ocorrer algum erro durante a criação
// do canal ou declaração da fila, a função registra um erro fatal e sai.
// Retorna um ponteiro para uma estrutura de servidor contendo a conexão e o canal do RabbitMQ.

func newServer() *server {
	// Estabelece conexão com o RabbitMQ, com até 5 tentativas e intervalo de 2 segundos entre elas
	conn := dialRabbitMQ(5, 2*time.Second)

	// Abre um canal de comunicação dentro da conexão com o RabbitMQ
	ch, err := conn.Channel()
	if err != nil {
		// Encerra a aplicação se não conseguir abrir o canal
		log.Fatalf("❌ Erro ao abrir canal do RabbitMQ: %v", err)
	}

	// Declara a fila "ping_events" ao iniciar o servidor
	_, err = ch.QueueDeclare(
		"ping_events", // nome da fila
		false,         // durable: false = a fila não sobrevive a reinícios do RabbitMQ
		false,         // autoDelete: false = a fila não será apagada automaticamente quando não tiver consumidores
		false,         // exclusive: false = a fila pode ser usada por múltiplas conexões
		false,         // noWait: false = espera a resposta do RabbitMQ na criação
		nil,           // argumentos extras (ex: TTL, dead-letter, etc.)
	)
	if err != nil {
		// Encerra a aplicação se não conseguir declarar a fila
		log.Fatalf("❌ Erro ao declarar fila: %v", err)
	}

	// Retorna uma instância do servidor com a conexão e canal do RabbitMQ já prontos para uso
	return &server{rabbitConn: conn, rabbitChan: ch}
}

// Ping lida com mensagens de solicitação de PingRequest recebidas, registra a mensagem recebida e
// a publica em uma fila do RabbitMQ. Ele responde com uma PingResponse contendo
// uma mensagem de pong e um carimbo de data/hora. Se ocorrer um erro durante a publicação,
// ele retorna um erro.

func (s *server) Ping(ctx context.Context, req *benchmarkpb.PingRequest) (*benchmarkpb.PingResponse, error) {
	// Loga a mensagem recebida do cliente para monitoramento e debug
	log.Printf("📩 Recebido: %s", req.GetMessage())

	// Monta a mensagem que será enviada para a fila do RabbitMQ
	msg := "Ping recebido: " + req.GetMessage()

	// Publica a mensagem na fila usando o método publishToQueue da instância do servidor
	err := s.publishToQueue(msg)
	if err != nil {
		// Loga o erro caso a publicação na fila falhe
		log.Printf("❌ Erro ao publicar no RabbitMQ: %v", err)

		// Retorna erro para o cliente, encapsulando o erro original
		return nil, fmt.Errorf("erro ao publicar mensagem no RabbitMQ: %w", err)
	}

	// Retorna a resposta gRPC para o cliente com a mensagem de resposta e timestamp atual em milissegundos
	return &benchmarkpb.PingResponse{
		Reply:     "pong: " + req.GetMessage(), // Resposta ao cliente
		Timestamp: time.Now().UnixMilli(),      // Marca o tempo da resposta
	}, nil
}

// Publica mensagem com retry simples (mutex garante uso seguro do canal)
func (s *server) publishToQueue(message string) error {
	// Garante que apenas uma goroutine por vez publique no canal do RabbitMQ (evita race condition)
	s.mu.Lock()
	defer s.mu.Unlock() // Libera o lock ao final da função, mesmo em caso de erro

	// Tenta publicar a mensagem até 3 vezes
	for i := 0; i < 3; i++ {
		// Publica a mensagem no canal RabbitMQ
		err := s.rabbitChan.Publish(
			"",            // Exchange vazio: usa default exchange do RabbitMQ
			"ping_events", // Routing key = nome da fila
			false,         // mandatory = false (não exige fila receptora ativa)
			false,         // immediate = false (não exige consumidor ativo no momento)
			amqp.Publishing{
				ContentType: "text/plain",    // Tipo de conteúdo (simples texto)
				Body:        []byte(message), // Corpo da mensagem como bytes
			},
		)

		// Se a publicação foi bem-sucedida, retorna nil (sem erro)
		if err == nil {
			return nil
		}

		// Se falhou, loga o erro e aguarda 2 segundos antes de tentar novamente
		log.Printf("⚠️  Tentativa %d: erro ao publicar, retry em 2s... (%v)", i+1, err)
		time.Sleep(2 * time.Second)
	}

	// Após 3 tentativas sem sucesso, retorna erro final
	return fmt.Errorf("falha ao publicar mensagem após retries")
}

// Conexão persistente com retry progressivo
func dialRabbitMQ(retries int, delay time.Duration) *amqp.Connection {
	// Declara as variáveis para a conexão e eventual erro
	var conn *amqp.Connection
	var err error

	// Tenta se conectar ao RabbitMQ pelo número de tentativas definido
	for i := 0; i < retries; i++ {
		// Tenta abrir uma conexão com o RabbitMQ
		conn, err = amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
		if err == nil {
			// Se conexão for bem-sucedida, loga e retorna
			log.Printf("✅ Conectado ao RabbitMQ com sucesso")
			return conn
		}

		// Loga a falha e o número da tentativa
		log.Printf("⏳ Tentativa %d de conexão falhou: %v", i+1, err)

		// Espera o tempo definido antes da próxima tentativa
		time.Sleep(delay)

		// Aumenta o tempo de espera para próxima tentativa (backoff exponencial simples)
		delay *= 2
	}

	// Se todas as tentativas falharem, encerra o programa com log de erro fatal
	log.Fatalf("❌ Falha ao conectar ao RabbitMQ após %d tentativas: %v", retries, err)

	// Retorno redundante, necessário apenas para compilação (o programa já terá sido encerrado)
	return nil
}

// main inicia o servidor gRPC que atende chamadas de PingRequest em :50051
// e as registra em uma fila do RabbitMQ.
func main() {
	// Abre um listener TCP na porta 50051, que será usada pelo servidor gRPC
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		// Se ocorrer um erro ao escutar na porta, o programa é encerrado com uma mensagem de erro
		log.Fatalf("❌ Erro ao escutar: %v", err)
	}

	// Cria uma nova instância do servidor, já com conexão e canal RabbitMQ inicializados
	srv := newServer()

	// Cria uma instância do servidor gRPC
	grpcServer := grpc.NewServer()

	// Registra a implementação do serviço `PingService` no servidor gRPC
	benchmarkpb.RegisterPingServiceServer(grpcServer, srv)

	// Ativa o suporte a reflection — permite ferramentas como `grpcurl` explorarem a API dinamicamente
	reflection.Register(grpcServer)

	// Informa no log que o servidor está ouvindo na porta especificada
	log.Println("🚀 Servidor gRPC ouvindo em :50051...")

	// Inicia o servidor gRPC e começa a aceitar conexões
	// Se ocorrer erro durante a execução, o programa é encerrado com log fatal
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("❌ Erro ao iniciar servidor: %v", err)
	}
}
