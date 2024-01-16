from flask import Flask, request, jsonify
from openai import OpenAI

import json
import openai
import os
import time

####################
# Configuration.
####################

# NOTE: Assistant ID should be set on the `BOT_ASSISTANT_ID` env var.
assistantID = os.getenv("BOT_ASSISTANT_ID", None)
openAIKey = os.getenv("BOT_OPENAI_API_KEY", None) or os.getenv("OPENAI_API_KEY", None)
openAIOrg = os.getenv("BOT_OPENAI_ORG_ID", None) or os.getenv("OPENAI_ORG_ID", None)

####################
# Utilities.
####################

def wait_on_run(thread_id: str, run_id: str):
    """
    Wait for a run to be completed.

    Args:
        thread_id (str): The thread ID.
        run_id (str): The run ID.

    Returns:
        The completed run.
    """

    while True:
        run = client.beta.threads.runs.retrieve(thread_id=thread_id, run_id=run_id)
        if run.status not in ["queued", "in_progress"]:
            return run
        time.sleep(0.5)

def print_json(obj, message=None):
    """
    Show a JSON object.

    Args:
        obj: The object to be shown.
        message: The message to be shown before the object.
    """

    print(f"{message + ': ' if message else ''}{json.dumps(json.loads(obj.model_dump_json()), indent=4)}\n")

def thread_retrieve_or_create(thread_id: str):
    """
    Retrieves the specified thread, or create a new one.

    Args:
        thread_id (str): The thread ID.

    Returns:
        The thread object.
    """

    try:
        thread = client.beta.threads.retrieve(thread_id)
    except openai.NotFoundError as err:
        print(f"Thread {thread_id} not found. Creating a new one...")

        thread = client.beta.threads.create()
    except Exception as err:
        print("Failed to retrieve thread: " + err.message)

    return thread

def message_create(thread_id:str, content:str, role="user"):
    """
    Create a message related to the specified thread.

    Args:
        thread_id (str): The thread ID.
        content (str): The message content.
        role (str, optional): The message role. Defaults to "user".

    Returns:
        The created message.
    """

    message = client.beta.threads.messages.create(thread_id=thread_id, role=role, content=content)

    return message

def message_convert_to(messages):
    """
    It converts OpenAI Messages' format to a more readable format.

    Args:
        messages: The messages to be processed.

    Returns:
        The processed messages.
    """

    processed_messages = []

    for message in messages:
        for content in message.content:
            processed_messages.append({
                "id": message.id,
                "role": message.role,
                "thread_id": message.thread_id,
                "value": content.text.value
            })

    return processed_messages

def message_submit(
    assistant_id:str,
    thread_id:str,
    message:str,
    after_message_id=None,
    order="asc",
):
    """
    Submit a message to the assistant and retrieve the response.

    Args:
        assistant_id (str): The assistant ID.
        thread_id (str): The thread ID.
        message (str): The message to be submitted.
        after_message_id (str, optional): The message ID to start the retrieval. Defaults to None.
        order (str, optional): The order of the messages. Defaults to "asc".

    Returns:
        A dict containing the the processed messages, and other useful information.
    """

    created_message = message_create(thread_id, message)

    completed_run = run_create_and_run(assistant_id, thread_id)

    # Retrieve all messages after the last user message
    raw_messages = client.beta.threads.messages.list(thread_id=thread_id, order=order, after=after_message_id)

    # Calculate the execution time
    execution_time = completed_run.completed_at - completed_run.created_at

    # return as a dict
    return {
        "completed_run": completed_run,
        "created_message": created_message,
        "execution_time": execution_time,
        "processed_messages": message_convert_to(raw_messages),
        "raw_messages": raw_messages
    }

def run_create_and_run(assistant_id:str, thread_id:str):
    """
    Create and run a thread.

    Args:
        assistant_id (str): The assistant ID.
        thread_id (str): The thread ID.

    Returns:
        The completed run.
    """

    run = client.beta.threads.runs.create(thread_id=thread_id, assistant_id=assistant_id)

    return wait_on_run(thread_id, run.id)

####################
# Application starts here.
####################

# Initialize the OpenAI client.
# - `api_key` is automatically inferred from the `OPENAI_API_KEY` env var.
# - `organization` is automatically inferred from the `OPENAI_ORG_ID` env var.
client = OpenAI(
    api_key=openAIKey,
    organization=openAIOrg
)

# Retrieves the already setup assistant.
assistant = client.beta.assistants.retrieve(assistantID)

####################
# API server starts here.
####################

# Initialize the Flask app.
app = Flask("openai-bot")

@app.route('/api/v1/message', methods=['POST'])
def put_message():
    data = request.json

    thread_id = data.get("thread_id", None)
    question = data.get("question", None)
    after_message_id = data.get("after_message_id", None)

    # Return error if thread_id AND question are not present.
    if not question:
        return jsonify({"error": "thread_id and question are required"}), 400

    # Retrieve the specified thread, or create a new one.
    thread = thread_retrieve_or_create(thread_id)

    # Submit the question and retrieve the response.
    response = message_submit(
        assistant.id,
        thread.id,
        question,
        after_message_id=after_message_id
    )

    return jsonify({
        "assistant_id": assistant.id,
        "thread_id": thread.id,
        "message_id": response["created_message"].id,
        "run_id": response["completed_run"].id,
        "execution_time": response["execution_time"],
        "messages": response["processed_messages"],
    }), 200

if __name__ == '__main__':
    app.run(debug=True, port=38234)