//line lex.rl:1
package msgpiler

import (
	"fmt"
	"strconv"
)

//line lex.go:12
const rosmsg_start int = 1
const rosmsg_first_final int = 1
const rosmsg_error int = 0

const rosmsg_en_main int = 1

//line lex.rl:14
type lexer struct {
	data        []byte
	p, pe, cs   int
	ts, te, act int
}

func newLexer(data []byte) *lexer {
	lex := &lexer{
		data: data,
		pe:   len(data),
	}

//line lex.go:35
	{
		lex.cs = rosmsg_start
		lex.ts = 0
		lex.te = 0
		lex.act = 0
	}

//line lex.rl:28
	return lex
}

func (lex *lexer) Lex(out *yySymType) int {
	eof := lex.pe
	tok := 0

//line lex.go:51
	{
		if (lex.p) == (lex.pe) {
			goto _test_eof
		}
		switch lex.cs {
		case 1:
			goto st_case_1
		case 0:
			goto st_case_0
		case 2:
			goto st_case_2
		case 3:
			goto st_case_3
		case 4:
			goto st_case_4
		case 5:
			goto st_case_5
		case 6:
			goto st_case_6
		case 7:
			goto st_case_7
		case 8:
			goto st_case_8
		case 9:
			goto st_case_9
		case 10:
			goto st_case_10
		case 11:
			goto st_case_11
		case 12:
			goto st_case_12
		case 13:
			goto st_case_13
		case 14:
			goto st_case_14
		case 15:
			goto st_case_15
		case 16:
			goto st_case_16
		case 17:
			goto st_case_17
		case 18:
			goto st_case_18
		case 19:
			goto st_case_19
		case 20:
			goto st_case_20
		case 21:
			goto st_case_21
		case 22:
			goto st_case_22
		case 23:
			goto st_case_23
		case 24:
			goto st_case_24
		case 25:
			goto st_case_25
		case 26:
			goto st_case_26
		case 27:
			goto st_case_27
		case 28:
			goto st_case_28
		case 29:
			goto st_case_29
		case 30:
			goto st_case_30
		case 31:
			goto st_case_31
		case 32:
			goto st_case_32
		case 33:
			goto st_case_33
		case 34:
			goto st_case_34
		case 35:
			goto st_case_35
		case 36:
			goto st_case_36
		case 37:
			goto st_case_37
		case 38:
			goto st_case_38
		case 39:
			goto st_case_39
		}
		goto st_out
	tr0:
//line lex.rl:58
		lex.te = (lex.p) + 1

		goto st1
	tr2:
//line lex.rl:50
		lex.te = (lex.p) + 1
		{
			tok = NEWLINE
			{
				(lex.p)++
				lex.cs = 1
				goto _out
			}
		}
		goto st1
	tr4:
//line lex.rl:57
		lex.te = (lex.p) + 1
		{
			tok = NAMESPACESEPARATOR
			{
				(lex.p)++
				lex.cs = 1
				goto _out
			}
		}
		goto st1
	tr9:
//line lex.rl:54
		lex.te = (lex.p) + 1
		{
			tok = LEFTBRACKET
			{
				(lex.p)++
				lex.cs = 1
				goto _out
			}
		}
		goto st1
	tr10:
//line lex.rl:55
		lex.te = (lex.p) + 1
		{
			tok = RIGHTBRACKET
			{
				(lex.p)++
				lex.cs = 1
				goto _out
			}
		}
		goto st1
	tr18:
//line lex.rl:48
		lex.te = (lex.p)
		(lex.p)--
		{
			tok = COMMENT
			{
				(lex.p)++
				lex.cs = 1
				goto _out
			}
		}
		goto st1
	tr19:
//line lex.rl:52
		lex.te = (lex.p)
		(lex.p)--
		{
			out.integerValue, _ = strconv.Atoi(string(lex.data[lex.ts:lex.te]))
			tok = INTEGER
			{
				(lex.p)++
				lex.cs = 1
				goto _out
			}
		}
		goto st1
	tr20:
//line lex.rl:49
		lex.te = (lex.p)
		(lex.p)--
		{
			tok = CONSTANT
			{
				(lex.p)++
				lex.cs = 1
				goto _out
			}
		}
		goto st1
	tr21:
//line NONE:1
		switch lex.act {
		case 4:
			{
				(lex.p) = (lex.te) - 1
				out.typeValue = string(lex.data[lex.ts:lex.te])
				tok = SIMPLETYPE
				{
					(lex.p)++
					lex.cs = 1
					goto _out
				}
			}
		case 6:
			{
				(lex.p) = (lex.te) - 1
				out.identifierValue = string(lex.data[lex.ts:lex.te])
				tok = IDENTIFIER
				{
					(lex.p)++
					lex.cs = 1
					goto _out
				}
			}
		}

		goto st1
	tr22:
//line lex.rl:53
		lex.te = (lex.p)
		(lex.p)--
		{
			out.identifierValue = string(lex.data[lex.ts:lex.te])
			tok = IDENTIFIER
			{
				(lex.p)++
				lex.cs = 1
				goto _out
			}
		}
		goto st1
	tr25:
//line lex.rl:56
		lex.te = (lex.p) + 1
		{
			tok = MSGDEF
			{
				(lex.p)++
				lex.cs = 1
				goto _out
			}
		}
		goto st1
	st1:
//line NONE:1
		lex.ts = 0

		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof1
		}
	st_case_1:
//line NONE:1
		lex.ts = (lex.p)

//line lex.go:216
		switch lex.data[(lex.p)] {
		case 10:
			goto tr2
		case 32:
			goto tr0
		case 35:
			goto st2
		case 47:
			goto tr4
		case 61:
			goto st4
		case 77:
			goto st6
		case 91:
			goto tr9
		case 93:
			goto tr10
		case 98:
			goto st9
		case 100:
			goto st14
		case 102:
			goto st21
		case 105:
			goto st28
		case 115:
			goto st32
		case 116:
			goto st37
		case 117:
			goto st39
		}
		switch {
		case lex.data[(lex.p)] < 48:
			if 9 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 13 {
				goto tr0
			}
		case lex.data[(lex.p)] > 57:
			switch {
			case lex.data[(lex.p)] > 90:
				if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
					goto tr7
				}
			case lex.data[(lex.p)] >= 65:
				goto tr7
			}
		default:
			goto st3
		}
		goto st0
	st_case_0:
	st0:
		lex.cs = 0
		goto _out
	st2:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof2
		}
	st_case_2:
		if lex.data[(lex.p)] == 10 {
			goto tr18
		}
		goto st2
	st3:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof3
		}
	st_case_3:
		if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
			goto st3
		}
		goto tr19
	st4:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof4
		}
	st_case_4:
		if lex.data[(lex.p)] == 10 {
			goto tr20
		}
		goto st4
	tr7:
//line NONE:1
		lex.te = (lex.p) + 1

//line lex.rl:53
		lex.act = 6
		goto st5
	tr29:
//line NONE:1
		lex.te = (lex.p) + 1

//line lex.rl:51
		lex.act = 4
		goto st5
	st5:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof5
		}
	st_case_5:
//line lex.go:317
		if lex.data[(lex.p)] == 95 {
			goto tr7
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr21
	st6:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof6
		}
	st_case_6:
		switch lex.data[(lex.p)] {
		case 83:
			goto st7
		case 95:
			goto tr7
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st7:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof7
		}
	st_case_7:
		switch lex.data[(lex.p)] {
		case 71:
			goto st8
		case 95:
			goto tr7
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st8:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof8
		}
	st_case_8:
		switch lex.data[(lex.p)] {
		case 58:
			goto tr25
		case 95:
			goto tr7
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st9:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof9
		}
	st_case_9:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 111:
			goto st10
		case 121:
			goto st12
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st10:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof10
		}
	st_case_10:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 111:
			goto st11
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st11:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof11
		}
	st_case_11:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 108:
			goto tr29
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st12:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof12
		}
	st_case_12:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 116:
			goto st13
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st13:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof13
		}
	st_case_13:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 101:
			goto tr29
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st14:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof14
		}
	st_case_14:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 117:
			goto st15
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st15:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof15
		}
	st_case_15:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 114:
			goto st16
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st16:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof16
		}
	st_case_16:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 97:
			goto st17
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 98 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st17:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof17
		}
	st_case_17:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 116:
			goto st18
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st18:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof18
		}
	st_case_18:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 105:
			goto st19
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st19:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof19
		}
	st_case_19:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 111:
			goto st20
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st20:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof20
		}
	st_case_20:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 110:
			goto tr29
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st21:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof21
		}
	st_case_21:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 108:
			goto st22
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st22:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof22
		}
	st_case_22:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 111:
			goto st23
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st23:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof23
		}
	st_case_23:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 97:
			goto st24
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 98 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st24:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof24
		}
	st_case_24:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 116:
			goto st25
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st25:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof25
		}
	st_case_25:
		switch lex.data[(lex.p)] {
		case 51:
			goto st26
		case 54:
			goto st27
		case 95:
			goto tr7
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st26:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof26
		}
	st_case_26:
		switch lex.data[(lex.p)] {
		case 50:
			goto tr29
		case 95:
			goto tr7
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st27:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof27
		}
	st_case_27:
		switch lex.data[(lex.p)] {
		case 52:
			goto tr29
		case 95:
			goto tr7
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st28:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof28
		}
	st_case_28:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 110:
			goto st29
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st29:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof29
		}
	st_case_29:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 116:
			goto st30
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st30:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof30
		}
	st_case_30:
		switch lex.data[(lex.p)] {
		case 49:
			goto st31
		case 51:
			goto st26
		case 54:
			goto st27
		case 56:
			goto tr29
		case 95:
			goto tr7
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st31:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof31
		}
	st_case_31:
		switch lex.data[(lex.p)] {
		case 54:
			goto tr29
		case 95:
			goto tr7
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st32:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof32
		}
	st_case_32:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 116:
			goto st33
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st33:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof33
		}
	st_case_33:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 114:
			goto st34
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st34:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof34
		}
	st_case_34:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 105:
			goto st35
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st35:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof35
		}
	st_case_35:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 110:
			goto st36
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st36:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof36
		}
	st_case_36:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 103:
			goto tr29
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st37:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof37
		}
	st_case_37:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 105:
			goto st38
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st38:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof38
		}
	st_case_38:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 109:
			goto st13
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st39:
		if (lex.p)++; (lex.p) == (lex.pe) {
			goto _test_eof39
		}
	st_case_39:
		switch lex.data[(lex.p)] {
		case 95:
			goto tr7
		case 105:
			goto st28
		}
		switch {
		case lex.data[(lex.p)] < 65:
			if 48 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 57 {
				goto tr7
			}
		case lex.data[(lex.p)] > 90:
			if 97 <= lex.data[(lex.p)] && lex.data[(lex.p)] <= 122 {
				goto tr7
			}
		default:
			goto tr7
		}
		goto tr22
	st_out:
	_test_eof1:
		lex.cs = 1
		goto _test_eof
	_test_eof2:
		lex.cs = 2
		goto _test_eof
	_test_eof3:
		lex.cs = 3
		goto _test_eof
	_test_eof4:
		lex.cs = 4
		goto _test_eof
	_test_eof5:
		lex.cs = 5
		goto _test_eof
	_test_eof6:
		lex.cs = 6
		goto _test_eof
	_test_eof7:
		lex.cs = 7
		goto _test_eof
	_test_eof8:
		lex.cs = 8
		goto _test_eof
	_test_eof9:
		lex.cs = 9
		goto _test_eof
	_test_eof10:
		lex.cs = 10
		goto _test_eof
	_test_eof11:
		lex.cs = 11
		goto _test_eof
	_test_eof12:
		lex.cs = 12
		goto _test_eof
	_test_eof13:
		lex.cs = 13
		goto _test_eof
	_test_eof14:
		lex.cs = 14
		goto _test_eof
	_test_eof15:
		lex.cs = 15
		goto _test_eof
	_test_eof16:
		lex.cs = 16
		goto _test_eof
	_test_eof17:
		lex.cs = 17
		goto _test_eof
	_test_eof18:
		lex.cs = 18
		goto _test_eof
	_test_eof19:
		lex.cs = 19
		goto _test_eof
	_test_eof20:
		lex.cs = 20
		goto _test_eof
	_test_eof21:
		lex.cs = 21
		goto _test_eof
	_test_eof22:
		lex.cs = 22
		goto _test_eof
	_test_eof23:
		lex.cs = 23
		goto _test_eof
	_test_eof24:
		lex.cs = 24
		goto _test_eof
	_test_eof25:
		lex.cs = 25
		goto _test_eof
	_test_eof26:
		lex.cs = 26
		goto _test_eof
	_test_eof27:
		lex.cs = 27
		goto _test_eof
	_test_eof28:
		lex.cs = 28
		goto _test_eof
	_test_eof29:
		lex.cs = 29
		goto _test_eof
	_test_eof30:
		lex.cs = 30
		goto _test_eof
	_test_eof31:
		lex.cs = 31
		goto _test_eof
	_test_eof32:
		lex.cs = 32
		goto _test_eof
	_test_eof33:
		lex.cs = 33
		goto _test_eof
	_test_eof34:
		lex.cs = 34
		goto _test_eof
	_test_eof35:
		lex.cs = 35
		goto _test_eof
	_test_eof36:
		lex.cs = 36
		goto _test_eof
	_test_eof37:
		lex.cs = 37
		goto _test_eof
	_test_eof38:
		lex.cs = 38
		goto _test_eof
	_test_eof39:
		lex.cs = 39
		goto _test_eof

	_test_eof:
		{
		}
		if (lex.p) == eof {
			switch lex.cs {
			case 2:
				goto tr18
			case 3:
				goto tr19
			case 4:
				goto tr20
			case 5:
				goto tr21
			case 6:
				goto tr22
			case 7:
				goto tr22
			case 8:
				goto tr22
			case 9:
				goto tr22
			case 10:
				goto tr22
			case 11:
				goto tr22
			case 12:
				goto tr22
			case 13:
				goto tr22
			case 14:
				goto tr22
			case 15:
				goto tr22
			case 16:
				goto tr22
			case 17:
				goto tr22
			case 18:
				goto tr22
			case 19:
				goto tr22
			case 20:
				goto tr22
			case 21:
				goto tr22
			case 22:
				goto tr22
			case 23:
				goto tr22
			case 24:
				goto tr22
			case 25:
				goto tr22
			case 26:
				goto tr22
			case 27:
				goto tr22
			case 28:
				goto tr22
			case 29:
				goto tr22
			case 30:
				goto tr22
			case 31:
				goto tr22
			case 32:
				goto tr22
			case 33:
				goto tr22
			case 34:
				goto tr22
			case 35:
				goto tr22
			case 36:
				goto tr22
			case 37:
				goto tr22
			case 38:
				goto tr22
			case 39:
				goto tr22
			}
		}

	_out:
		{
		}
	}

//line lex.rl:62
	return tok
}

func (lex *lexer) Error(e string) {
	fmt.Println("Lexer Error method error:", e)
}
