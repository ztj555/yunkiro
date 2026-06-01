import { detectPhoneNumbers, DetectedNumber } from '../content/numberDetector';

function createTextNode(text: string, parentTag = 'P'): Text {
  const parent = document.createElement(parentTag);
  const textNode = document.createTextNode(text);
  parent.appendChild(textNode);
  document.body.appendChild(parent);
  return textNode;
}

describe('numberDetector', () => {
  beforeEach(() => {
    document.body.innerHTML = '';
  });

  describe('detectPhoneNumbers', () => {
    it('should detect a valid Chinese mobile number', () => {
      const div = document.createElement('div');
      div.textContent = 'Call me at 13800138000 for details';
      document.body.appendChild(div);

      const results = detectPhoneNumbers(document.body);
      expect(results).toHaveLength(1);
      expect(results[0].number).toBe('13800138000');
    });

    it('should detect multiple numbers in same text', () => {
      const div = document.createElement('div');
      div.textContent = 'Phone: 13800138000 or 15912345678';
      document.body.appendChild(div);

      const results = detectPhoneNumbers(document.body);
      expect(results).toHaveLength(2);
      expect(results[0].number).toBe('13800138000');
      expect(results[1].number).toBe('15912345678');
    });

    it('should detect numbers starting with different valid prefixes', () => {
      const validPrefixes = ['13', '14', '15', '16', '17', '18', '19'];
      for (const prefix of validPrefixes) {
        document.body.innerHTML = '';
        const div = document.createElement('div');
        div.textContent = `Number: ${prefix}900000000`;
        document.body.appendChild(div);

        const results = detectPhoneNumbers(document.body);
        expect(results).toHaveLength(1);
        expect(results[0].number).toBe(`${prefix}900000000`);
      }
    });

    it('should reject numbers starting with 10, 11, or 12', () => {
      const div = document.createElement('div');
      div.textContent = 'Not valid: 10800138000 11800138000 12800138000';
      document.body.appendChild(div);

      const results = detectPhoneNumbers(document.body);
      expect(results).toHaveLength(0);
    });

    it('should reject numbers that are part of longer sequences', () => {
      const div = document.createElement('div');
      div.textContent = 'ID: 123456789013800138000';
      document.body.appendChild(div);

      const results = detectPhoneNumbers(document.body);
      expect(results).toHaveLength(0);
    });

    it('should reject numbers followed by more digits', () => {
      const div = document.createElement('div');
      div.textContent = 'Number: 138001380001';
      document.body.appendChild(div);

      const results = detectPhoneNumbers(document.body);
      expect(results).toHaveLength(0);
    });

    it('should not detect numbers inside script tags', () => {
      const script = document.createElement('script');
      script.textContent = 'var phone = "13800138000";';
      document.body.appendChild(script);

      const results = detectPhoneNumbers(document.body);
      expect(results).toHaveLength(0);
    });

    it('should not detect numbers inside style tags', () => {
      const style = document.createElement('style');
      style.textContent = '.phone-13800138000 { color: red; }';
      document.body.appendChild(style);

      const results = detectPhoneNumbers(document.body);
      expect(results).toHaveLength(0);
    });

    it('should not detect numbers inside input elements', () => {
      const input = document.createElement('input');
      input.value = '13800138000';
      document.body.appendChild(input);

      const results = detectPhoneNumbers(document.body);
      expect(results).toHaveLength(0);
    });

    it('should not detect numbers inside textarea elements', () => {
      const textarea = document.createElement('textarea');
      textarea.textContent = '13800138000';
      document.body.appendChild(textarea);

      const results = detectPhoneNumbers(document.body);
      expect(results).toHaveLength(0);
    });

    it('should not detect numbers inside code/pre elements', () => {
      const code = document.createElement('code');
      code.textContent = '13800138000';
      document.body.appendChild(code);

      const pre = document.createElement('pre');
      pre.textContent = '15900000000';
      document.body.appendChild(pre);

      const results = detectPhoneNumbers(document.body);
      expect(results).toHaveLength(0);
    });

    it('should handle short text that cannot contain a phone number', () => {
      const div = document.createElement('div');
      div.textContent = '1380';
      document.body.appendChild(div);

      const results = detectPhoneNumbers(document.body);
      expect(results).toHaveLength(0);
    });

    it('should detect number at start of text', () => {
      const div = document.createElement('div');
      div.textContent = '13800138000 is my phone';
      document.body.appendChild(div);

      const results = detectPhoneNumbers(document.body);
      expect(results).toHaveLength(1);
      expect(results[0].startOffset).toBe(0);
    });

    it('should detect number at end of text', () => {
      const div = document.createElement('div');
      div.textContent = 'My phone is 13800138000';
      document.body.appendChild(div);

      const results = detectPhoneNumbers(document.body);
      expect(results).toHaveLength(1);
      expect(results[0].number).toBe('13800138000');
    });

    it('should detect numbers separated by non-digit chars', () => {
      const div = document.createElement('div');
      div.textContent = 'A: 13800138000, B: 15900000000';
      document.body.appendChild(div);

      const results = detectPhoneNumbers(document.body);
      expect(results).toHaveLength(2);
    });
  });
});
