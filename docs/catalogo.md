# Catálogo de provedores

Um **provedor** é uma imagem Docker mais o schema dos campos que ela aceita. É
daqui que o wizard tira o formulário — o frontend não conhece nenhuma variável
de nenhum jogo.

Tudo vive em [`api/internal/catalog/providers.go`](../api/internal/catalog/providers.go).

## O que está no catálogo

| Jogo | Imagem | RAM padrão / mínima | Portas |
|---|---|---|---|
| Minecraft (Java) | `itzg/minecraft-server` | 4g / 2g | 25565/tcp |
| Minecraft (Bedrock) | `itzg/minecraft-bedrock-server` | 2g / 1g | 19132/udp |
| Palworld | `thijsvanloef/palworld-server-docker` | 12g / 8g | 8211/udp, 27015/udp, 25575/tcp |
| Valheim | `lloesche/valheim-server` | 4g / 2g | 2456-2457/udp |
| ARK | `hermsi/ark-server` | 8g / 6g | 7777-7778/udp, 27015/udp, 27020/tcp |
| Terraria | `ryshe/terraria` | 2g / 512m | 7777/tcp |
| Factorio | `factoriotools/factorio` | 4g / 1g | 34197/udp, 27015/tcp |
| Satisfactory | `wolveix/satisfactory-server` | 12g / 8g | 7777/udp |
| Imagem custom | qualquer | 2g / 256m | você define |

O provedor `custom` é o único que aceita variáveis fora do schema — é a saída
para uma imagem que o catálogo ainda não descreve.

## Estado de verificação dos schemas

Os schemas foram escritos a partir da documentação de cada imagem, mas nem
todos foram exercitados contra um container de verdade. Antes de confiar num
campo, confira contra o `Docs` do provedor:

- **Conferidos na prática:** `itzg/minecraft-server`.
- **Da documentação, não testados:** os demais. Os mais prováveis de terem
  divergido são **ARK** e **Satisfactory**, onde os nomes de variável mudaram
  entre versões da imagem.

Campo com nome errado não quebra o `up`: a variável simplesmente é ignorada
pelo servidor de jogo, e o efeito é a configuração não pegar. Se um campo não
surtir efeito, esse é o primeiro lugar a olhar.

Nada disso impede usar o jogo — o provedor `custom` sempre aceita a imagem com
as variáveis digitadas à mão.

## Adicionando um provedor

1. Acrescente uma entrada em `providers` no `providers.go`.
2. `go test ./internal/catalog/` — `TestAllProvidersAreUsable` já cobre o
   básico: identificação completa, volume de mundo marcado, chaves sem
   repetição, enum com opções, e **todo default passando na própria
   validação** (um default fora do schema faria a instância nascer inválida sem
   o usuário ter tocado em nada).
3. Nada muda no frontend.

Os campos que merecem atenção:

- **`secret: true`** em qualquer senha. É o que mantém o valor fora do
  `docker-compose.yml`.
- **`minMemory`** honesto. É o que o painel usa para recusar uma instância que
  morreria com `Exited (137)`.
- **`stopGraceSeconds`** generoso em jogo que salva ao desligar. Minecraft e
  ARK corrompem save se levarem SIGKILL no meio da gravação; 120 s e 180 s são
  os valores usados hoje.
- **`optional: true`** em porta de RCON e de query. Elas não são publicadas por
  padrão — publicar RCON sem necessidade é abrir um console remoto do servidor.
- **`advanced: true`** no que quase ninguém mexe, para o formulário não
  assustar.

## Como a validação funciona

`Provider.Validate` aplica os defaults, confere tipo, faixa e enum, e junta
**todos** os problemas antes de devolver — o formulário marca os campos errados
de uma vez em vez de obrigar a salvar várias vezes para descobrir o resto.
