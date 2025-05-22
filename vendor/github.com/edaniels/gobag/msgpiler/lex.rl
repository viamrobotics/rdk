package msgpiler

import (
        "fmt"
        "strconv"
)

%%{ 
    machine rosmsg;
    write data;
    access lex.;
    variable p lex.p;
    variable pe lex.pe;
}%%

type lexer struct {
    data []byte
    p, pe, cs int
    ts, te, act int
}

func newLexer(data []byte) *lexer {
    lex := &lexer{ 
        data: data,
        pe: len(data),
    }
    %% write init;
    return lex
}

func (lex *lexer) Lex(out *yySymType) int {
    eof := lex.pe
    tok := 0
    %%{ 
        comment = '#' (any - '\n')* ;
        constant = '=' (any - '\n')* ;
        newline = '\n';
        simpletype = ('bool'|'byte'|'int8'|'uint8'|'int16'|'uint16'|'int32'|'uint32'|'int64'|'uint64'|'float32'|'float64'|'string'|'time'|'duration');
        integer = digit+;
        identifier = [a-zA-Z][a-zA-Z0-9_]*;
        leftbracket = '[';
        rightbracket = ']';
        msgdef = 'MSG:';
        namespaceseparator = '/';


        main := |*
            comment => { tok = COMMENT; fbreak; };
            constant => { tok = CONSTANT; fbreak; };
            newline => { tok = NEWLINE; fbreak; };
            simpletype => { out.typeValue = string(lex.data[lex.ts:lex.te]); tok = SIMPLETYPE; fbreak; };
            integer => { out.integerValue, _ = strconv.Atoi(string(lex.data[lex.ts:lex.te])); tok = INTEGER; fbreak; };
            identifier => { out.identifierValue = string(lex.data[lex.ts:lex.te]); tok = IDENTIFIER; fbreak; };
            leftbracket => { tok = LEFTBRACKET; fbreak; };
            rightbracket => { tok = RIGHTBRACKET; fbreak; };
            msgdef => { tok = MSGDEF; fbreak; };
            namespaceseparator => { tok = NAMESPACESEPARATOR; fbreak; };
            space;
        *|;

         write exec;
    }%%
    return tok;
}

func (lex *lexer) Error(e string) {
    fmt.Println("Lexer Error method error:", e)
}
