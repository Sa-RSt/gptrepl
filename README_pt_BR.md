# gptrepl
Ferramenta de linha de comando Read Eval Print Loop (REPL) para interagir manualmente com a API de chat da OpenAI. Esse repositório e ferramenta não estão afiliados à OpenAI.

## Introdução
Esse programa é uma interface baseada em Go para a API de chat da OpenAI (também conhecida como ChatGPT). O seu principal propósito é permitir uma experiência de usuário mais flexível.

## Usando o programa

### Executável binário

1. Faça o download e extraia um executável da seção "[Releases](https://github.com/Sa-RSt/gptrepl/releases)".
1. (Opcional) Adicione-o a algum caminho presente na variável PATH.
1. Teste:
```bash
gptrepl -help
```

### Compilar/executar a partir do código fonte
1. Verifique se o Go está corretamente instalado:
```bash
go version
```
Esse projeto foi testado em Go 1.23.0.

2. Clone o repositório:
```bash
git clone https://github.com/Sa-RSt/gptrepl.git
```
3. Navegue até o diretório do repositório:
```bash
cd gptrepl
```
4. Execute o pacote main:
```bash
go run .
```

## Obtendo uma chave da API da OpenAI
Uma chave da API é necessária para se comunicar com os modelos de linguagem da OpenAI. Siga essas etapas para obter uma, caso ainda não a possua:

1. Accesse https://platform.openai.com/settings/profile?tab=api-keys
1. Crie ou entre em sua conta.
1. Clique em "Criar nova chave secreta".
1. Guarde-a em um lugar seguro.

## Usando a chave da API
### Opção 1: Variável de ambiente
Defina a variável OPENAI_API_KEY antes de executar gptrepl:
```bash
OPENAI_API_KEY=sua-chave-aqui gptrepl
```
Também é possível definir a variável globalmente, com um risco maior de ter a chave exposta a outros programas.

### Opção 2: Argumento na linha de comando
Defina a chave da API como um parâmetro:
```bash
gptrepl -apikey sua-chave-aqui
```

### Opção 3: Guarde uma chave padrão no seu diretório de usuário
Se $OPENAI_API_KEY estiver indefinida e nenhuma chave de API tiver sido passada como parâmetro, gptrepl lerá a chave do seu diretório de usuário (e.g. /home/johndoe, C:\\Users\\johndoe). Para configurar isso, execute:
```bash
echo sua-chave-aqui > seu-diretório-de-usuário-aqui/.gptrepl-key
```

## Usando o programa
### Como uma shell interativa
Simplesmente execute:
```bash
gptrepl
```
E verá o seguinte prompt:
```
Enter "/help" for a list of commands.
(gpt-4)>
```
Você pode fazer perguntas diretamente:
```
(gpt-4)> Qual é o país com o maior território na América do Sul?
O país com o maior território na América do Sul é o Brasil.
```
E também pode usar comandos para manipular o contexto da conversa:
```
(gpt-4)> Qual é o país com o maior território na América do Sul?
O país com o maior território na América do Sul é o Brasil.
(gpt-4)> /print
[user]
Qual é o país com o maior território na América do Sul?

[assistant]
O país com o maior território na América do Sul é o Brasil.

(gpt-4)> /prepend system Você deve responder utilizando exatamente uma palavra.
(gpt-4)> /pop 1
(gpt-4)> /print
[system]
Você deve responder utilizando exatamente uma palavra.

[user]
Qual é o país com o maior território na América do Sul?

(gpt-4)> /send
Brasil
```
Use `/help` para listar todos os comandos.

### Uso de exemplo para processamento automático
```bash
$ cat verificador_de_fatos.json
[
    {
        "role": "system",
        "content": "Você é um verificador de fatos"
    },
    {
        "role": "system",
        "content": "Você só pode responder \"Sim\" se sabe que uma afirmação é verdadeira, \"Não\" se sabe que é falsa ou \"Impossível\" se não há informação suficiente."
    },
    {
        "role": "user",
        "content": "Paris é a capital da França?"
    },
    {
        "role": "assistant",
        "content": "Sim"
    }
]

$ echo "Crie 15 perguntas no estilo verdadeiro/falso para testar o conhecimento de mundo geral de uma pessoa, separadas somente por quebras de linha. Não inclua as respostas de forma alguma." | gptrepl -quiet -nocommands -model gpt-4o | tee perguntas.txt | gptrepl -quiet -nocommands -ctx verificador_de_fatos.json -model gpt-4o -forgetful > respostas.txt

$ cat perguntas.txt
1. O Egito está localizado na América do Sul.
2. A moeda oficial do Japão é o Euro.
3. A Torre Eiffel está situada em Paris.
4. A Grande Muralha da China pode ser vista do espaço a olho nu.
5. A Austrália é o menor continente do mundo.
6. A Guerra Fria foi um conflito militar direto entre os Estados Unidos e a União Soviética.
7. O idioma oficial do Brasil é o espanhol.
8. A Mona Lisa é um quadro pintado por Leonardo da Vinci.
9. A capital do Canadá é Toronto.
10. O monte Everest é a montanha mais alta do mundo.
11. A Antártida é habitada permanentemente por seres humanos.
12. A União Europeia é formada por 27 países-membros.
13. A Igreja da Sagrada Família está localizada em Madrid.
14. Os Jogos Olímpicos da era moderna foram realizados pela primeira vez na Grécia em 1896.
15. Marte é o planeta mais próximo ao Sol.

$ cat respostas.txt
Não
Não
Sim
Não
Sim
Não
Não
Sim
Não
Sim
Não
Sim
Não
Sim
Não
```
**Explicação:** O extenso comando acima pode ser dividido em duas partes. Exploremos a primeira:
```bash
echo "Crie 15 perguntas no estilo verdadeiro/falso para testar o conhecimento de mundo geral de uma pessoa, separadas somente por quebras de linha. Não inclua as respostas de forma alguma." | gptrepl -quiet -nocommands -model gpt-4o | tee perguntas.txt
```
O acima faz o gptrepl responder a apenas uma pergunta, escrever a saída do modelo para `perguntas.txt` e também passar adiante essa saída para a segunda parte do comando, explicada abaixo. A flag `-quiet` previne qualquer saída além de comandos como `/print` e o próprio modelo, enquanto a flag `-nocommands` desabilita comandos interativos completamente. Use o parâmetro `-model` para especificar outro modelo a ser usado (padrão: gpt-4). 
```bash
| gptrepl -quiet -nocommands -ctx verificador_de_fatos.json -model gpt-4o -forgetful > respostas.txt
```
A segunda parte lê a saída da primeira parte (uma pergunta por linha) e responde a cada pergunta, usando o contexto dado pela flag `ctx`, e grava as respostas em `answers.txt`. A flag `-forgetful` impede as perguntas e respostas de serem adicionadas ao contexto da conversa, já que isso geraria um acúmulo de informações no contexto, aumentando, sem necessidade, o número de tokens enviados.
