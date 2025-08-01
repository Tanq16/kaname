#!/bin/bash

cd /app/scripts

if [ ! -d "venv" ]; then
    echo "Creating virtual environment..."
    python3 -m venv venv
else
    echo "Virtual environment already exists."
fi

. venv/bin/activate
pip install requests matplotlib pandas seaborn

echo "Cold start complete!"
