import * as Utils from './utils'

test('split2', () => {
  ;[
    {input: `1abc def`, output: ['1abc', 'def']},
    {input: `2abc "def asdf" jkl`, output: ['2abc', 'def asdf', 'jkl']},
    {input: `3abc "it's it"`, output: ['3abc', "it's it"]},
    {input: `4abc"it's it"`, output: ["4abcit's it"]},
    {input: `5abc  it's it`, output: ['5abc', "it's", 'it']},
    {input: `6abc it's "it it"`, output: ['6abc', "it's", 'it it']},
    {input: `7abc 'abc "it it"'`, output: ['7abc', `abc "it it"`]},
  ].forEach(({input, output}) => expect(Utils.split2(input)).toEqual(output))
})

