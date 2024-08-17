# gptrepl
Simple Go-based command-line Read Eval Print Loop (REPL) client for manually interacting with the OpenAI Chat Completions API, allowing for a more flexible and low-level experience. This repository and this tool are not affiliated with OpenAI.

## Getting started

### Standalone executable

1. Download a binary file from this repository's "Releases" section.
1. (Optional) Put it somewhere in your PATH.
1. Test it:
```bash
gptrepl -help
```

### Running from source
1. Ensure you have go installed:
```bash
go version
```
This project was tested in Go 1.23.0.

2. Clone the repository:
```bash
git clone https://github.com/Sa-RSt/gptrepl.git
```
3. Navigate into the repository:
```bash
cd gptrepl
```
4. Run the main package:
```bash
go run .
```

## Getting an OpenAI API Key
An API key is necessary in order for this program to communicate with OpenAI's chat models. Follow these steps to obtain one if you haven't already:

1. Access https://platform.openai.com/settings/profile?tab=api-keys
1. Create or log into your account.
1. Click on "Create new secret key".
1. Store the key somewhere safe.

## Using the API key
### Option 1: Environment variable
Set the OPENAI_API_KEY environment variable before running gptrepl:
```bash
OPENAI_API_KEY=your-key-here gptrepl
```
You may also set this variable globally, at a greater risk of exposing your key to other programs.

### Option 2: Command line argument
Set the API key as an argument:
```bash
gptrepl -apikey your-key-here
```

### Option 3: Store a default key in your home directory
If $OPENAI_API_KEY is unset and an API key wasn't given as a command line argument, gptrepl will read the key from your user home directory (e.g. /home/johndoe, C:\\Users\\johndoe). To set this up, run:
```bash
echo your-key-here > your-home-directory/.gptrepl-key
```

## Using the program
### As an interactive shell
Just run:
```bash
gptrepl
```
And you should see the following prompt:
```
Enter "/help" for a list of commands.
(gpt-4)>
```
You can ask questions directly:
```
(gpt-4)> What is the country with the largest territory in South America?
The country with the largest territory in South America is Brazil.
```
And you can use commands to manipulate the conversation context:
```
(gpt-4)> What is the country with the largest territory in South America?
The country with the largest territory in South America is Brazil.
(gpt-4)> /print
[user]
What is the country with the largest territory in South America?

[assistant]
The country with the largest territory in South America is Brazil.

(gpt-4)> /prepend system You must answer with exactly one word.
(gpt-4)> /pop 1
(gpt-4)> /print
[system]
You must answer with exactly one word.

[user]
What is the country with the largest territory in South America?

(gpt-4)> /send
Brazil
```
Use `/help` to check out other commands.

### For automated processing: an example usage
```bash
$ cat fact_verifier.json
[
    {
        "role": "system",
        "content": "You are a fact verifier"
    },
    {
        "role": "system",
        "content": "You can only answer \"Yes\" if you know a statement is true, \"No\" if you know it's untrue or \"Undecidable\" if there is not enough information."
    },
    {
        "role": "user",
        "content": "Is Paris the capital of France?"
    },
    {
        "role": "assistant",
        "content": "Yes"
    }
]

$ echo "Generate 15 true/false questions to test a person's general knowledge of the world, separated only by one newline. Do not include the answers in any form."| gptrepl -quiet -nocommands -model gpt-4o | tee questions.txt | gptrepl -quiet -nocommands -ctx fact_verifier.json -model gpt-4o -forgetful > answers.txt

$ cat questions.txt
1. The Great Wall of China is visible from space.
2. Australia is both a country and a continent.
3. Mount Everest is located in India.
4. The Amazon River is the longest river in the world.
5. The capital of Canada is Ottawa.
6. The Sahara Desert is the largest desert in the world.
7. Tokyo is the most populous city in the world.
8. The Nile River flows through South America.
9. The currency of Japan is the Yen.
10. Russia is the largest country in the world by land area.
11. The United Nations headquarters is located in Paris.
12. The official language of Brazil is Spanish.
13. Antarctica is inhabited by humans all year round.
14. The Eiffel Tower is located in Berlin.
15. Mount Kilimanjaro is in Tanzania.

$ cat answers.txt
No
Yes
No
No
Yes
Undecidable
Yes
No
Yes
Yes
No
No
No
No
Yes
```
**Explanation:** The large command above can be divided into two parts. Let's explore the first one:
```bash
echo "Generate 15 true/false questions to test a person's general knowledge of the world, separated only by one newline. Do not include the answers in any form." | gptrepl -quiet -nocommands -model gpt-4o | tee questions.txt
```
The above makes gptrepl answer only one question, write the model's output to `questions.txt` and also pipe the output to the second part of the large command, which is be explained below. The `-quiet` flag prevents any output other than commands such as `/print` and the model itself, while the `-nocommands` flag disables interactive commands entirely. Use the `-model` flag to specify a model other than the default (gpt-4).
```bash
| gptrepl -quiet -nocommands -ctx fact_verifier.json -model gpt-4o -forgetful > answers.txt
```
The second part takes the piped output (one question per line) and answers each question using the context given by the `-ctx` flag and writes the answers to `answers.txt`. The `-forgetful` flag prevents each question and answer from being added to the conversation context, which saves tokens by avoiding excess information exchange.
