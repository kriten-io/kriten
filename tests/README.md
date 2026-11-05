# Running Tests:

## Install deps (virtualenv recommended)
python3 -m venv .venv && source .venv/bin/activate
pip install -r tests/requirements.txt

## Run all tests
export KRITEN_URL="http://" 
export KRITEN_USERNAME="" 
export KRITEN_PASSWORD="" 

pytest tests/ -v

##Smoke tests only
pytest tests/ -m smoke -v

## CRUD tests only
pytest tests/ -m crud -v

## Negative tests only
pytest tests/ -m negative -v