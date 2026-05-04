#!/usr/bin/env python3
"""Kokoro TTS CLI bridge for ClawBench.
Reads text from stdin, writes WAV audio to the specified output file.

Supports both v1.0 (English) and v1.1-zh (Chinese) ONNX models.
For v1.1-zh models, uses misaki[zh] G2P for superior Chinese phonemization
(espeak-ng's cmn phonemizer mispronounces polyphonic characters and has a
foreign accent). Falls back to espeak-ng if misaki is unavailable.

Auto-detects model version and patches kokoro-onnx speed dtype bug for v1.1.

Usage: echo "text" | kokoro_tts.py --model <onnx_path> --voices <voices_bin> --voice <voice> --output <wav_path> [--lang <lang>] [--speed <speed>]
"""
import argparse
import os
import sys
import time

import numpy as np
import soundfile as sf
from kokoro_onnx import Kokoro, MAX_PHONEME_LENGTH, SAMPLE_RATE


def patched_create_audio(kokoro, phonemes, voice, speed):
    """Fixed _create_audio that uses float32 speed for v1.1 models.

    kokoro-onnx 0.5.0 has a bug: when model uses 'input_ids' input,
    it sets speed dtype to int32, but v1.1 models expect float32.
    """
    if len(phonemes) > MAX_PHONEME_LENGTH:
        phonemes = phonemes[:MAX_PHONEME_LENGTH]

    start_t = time.time()
    tokens = np.array(kokoro.tokenizer.tokenize(phonemes), dtype=np.int64)
    voice_data = voice[len(tokens)]
    tokens_arr = [[0, *tokens, 0]]

    inputs = {
        "input_ids": tokens_arr,
        "style": np.array(voice_data, dtype=np.float32).reshape(1, 256),
        "speed": np.array([speed], dtype=np.float32),  # Fix: float32, not int32
    }

    audio = kokoro.sess.run(None, inputs)[0]
    return audio, SAMPLE_RATE


def is_v11_model(kokoro):
    """Check if the model is v1.1+ (uses 'input_ids' input)."""
    input_names = [inp.name for inp in kokoro.sess.get_inputs()]
    return "input_ids" in input_names


def try_load_misaki_zh(model_path):
    """Try to load misaki zh G2P and the corresponding vocab config.

    Returns (g2p, vocab_config_path) on success, (None, None) on failure.
    """
    try:
        from misaki import zh
        g2p = zh.ZHG2P(version="1.1")

        # Look for vocab config next to the model file
        model_dir = os.path.dirname(model_path)
        vocab_config = os.path.join(model_dir, "config.json")
        if not os.path.isfile(vocab_config):
            return None, None

        return g2p, vocab_config
    except (ImportError, Exception) as e:
        print(f"misaki[zh] not available, falling back to espeak-ng: {e}", file=sys.stderr)
        return None, None


def main():
    parser = argparse.ArgumentParser(description="Kokoro TTS synthesis")
    parser.add_argument("--model", required=True, help="Path to kokoro ONNX model file")
    parser.add_argument("--voices", required=True, help="Path to voices bin/npz file")
    parser.add_argument("--voice", required=True, help="Voice name (e.g. zf_001, zm_010, zf_xiaobei)")
    parser.add_argument("--output", required=True, help="Output WAV file path")
    parser.add_argument("--lang", default="cmn", help="Language code (default: cmn for Mandarin Chinese)")
    parser.add_argument("--speed", type=float, default=1.0, help="Speech speed multiplier (default: 1.0)")
    args = parser.parse_args()

    # Read text from stdin
    text = sys.stdin.read().strip()
    if not text:
        print("Error: no text provided on stdin", file=sys.stderr)
        sys.exit(1)

    # For v1.1-zh Chinese models: prefer misaki[zh] G2P over espeak-ng
    # misaki correctly handles polyphonic characters (多音字) and produces
    # much more natural Chinese pronunciation than espeak-ng's cmn phonemizer.
    g2p, vocab_config = None, None
    if args.lang in ("cmn", "zh", "z", "zh-CN", "zh-TW"):
        g2p, vocab_config = try_load_misaki_zh(args.model)

    # Initialize Kokoro
    kokoro_kwargs = {"model_path": args.model, "voices_path": args.voices}
    if vocab_config:
        kokoro_kwargs["vocab_config"] = vocab_config
    kokoro = Kokoro(**kokoro_kwargs)

    # Patch _create_audio for v1.1+ models (speed dtype bug)
    if is_v11_model(kokoro):
        kokoro._create_audio = lambda phonemes, voice, speed: patched_create_audio(
            kokoro, phonemes, voice, speed
        )

    # Synthesize using the best available phonemization path
    if g2p and vocab_config:
        phonemes, _ = g2p(text)
        samples, sample_rate = kokoro.create(
            phonemes, voice=args.voice, speed=args.speed, is_phonemes=True
        )
        print(f"phonemization: misaki[zh] (v1.1 G2P)", file=sys.stderr)
    else:
        samples, sample_rate = kokoro.create(
            text, voice=args.voice, speed=args.speed, lang=args.lang
        )
        print(f"phonemization: espeak-ng (lang={args.lang})", file=sys.stderr)

    # Write output
    sf.write(args.output, samples, sample_rate)
    print(f"OK: {args.output} ({len(samples)} samples, {sample_rate}Hz)", file=sys.stderr)


if __name__ == "__main__":
    main()
