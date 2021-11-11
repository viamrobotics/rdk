ace.define("ace/mode/puppet_highlight_rules",["require","exports","module","ace/lib/oop","ace/mode/text_highlight_rules"],function(e,t,n){"use strict";var r=e("../lib/oop"),i=e("./text_highlight_rules").TextHighlightRules,s=function(){this.$rules={start:[{token:["keyword.type.puppet","constant.class.puppet","keyword.inherits.puppet","constant.class.puppet"],regex:'^\\s*(class)(\\s+(?:[-_A-Za-z0-9".]+::)*[-_A-Za-z0-9".]+\\s*)(?:(inherits\\s*)(\\s+(?:[-_A-Za-z0-9".]+::)*[-_A-Za-z0-9".]+\\s*))?'},{token:["storage.function.puppet","name.function.puppet","punctuation.lpar"],regex:"(^\\s*define)(\\s+[a-zA-Z0-9_:]+\\s*)(\\()",push:[{token:"punctuation.rpar.puppet",regex:"\\)",next:"pop"},{include:"constants"},{include:"variable"},{include:"strings"},{include:"operators"},{defaultToken:"string"}]},{token:["language.support.class","keyword.operator"],regex:"\\b([a-zA-Z_]+)(\\s+=>)"},{token:["exported.resource.puppet","keyword.name.resource.puppet","paren.lparen"],regex:"(\\@\\@)?(\\s*[a-zA-Z_]*)(\\s*\\{)"},{token:"qualified.variable.puppet",regex:"(\\$([a-z][a-z0-9_]*)?(::[a-z][a-z0-9_]*)*::[a-z0-9_][a-zA-Z0-9_]*)"},{token:"singleline.comment.puppet",regex:"#(.)*$"},{token:"multiline.comment.begin.puppet",regex:"^\\s*\\/\\*",push:"blockComment"},{token:"keyword.control.puppet",regex:"\\b(case|if|unless|else|elsif|in|default:|and|or)\\s+(?!::)"},{token:"keyword.control.puppet",regex:"\\b(import|default|inherits|include|require|contain|node|application|consumes|environment|site|function|produces)\\b"},{token:"support.function.puppet",regex:"\\b(lest|str2bool|escape|gsub|Timestamp|Timespan|with|alert|crit|debug|notice|sprintf|split|step|strftime|slice|shellquote|type|sha1|defined|scanf|reverse_each|regsubst|return|emerg|reduce|err|failed|fail|versioncmp|file|generate|then|info|realize|search|tag|tagged|template|epp|warning|hiera_include|each|assert_type|binary_file|create_resources|dig|digest|filter|lookup|find_file|fqdn_rand|hiera_array|hiera_hash|inline_epp|inline_template|map|match|md5|new|next)\\b"},{token:"constant.types.puppet",regex:"\\b(String|File|Package|Service|Class|Integer|Array|Catalogentry|Variant|Boolean|Undef|Number|Hash|Float|Numeric|NotUndef|Callable|Optional|Any|Regexp|Sensitive|Sensitive.new|Type|Resource|Default|Enum|Scalar|Collection|Data|Pattern|Tuple|Struct)\\b"},{token:"paren.lparen",regex:"[[({]"},{token:"paren.rparen",regex:"[\\])}]"},{include:"variable"},{include:"constants"},{include:"strings"},{include:"operators"},{token:"regexp.begin.string.puppet",regex:"\\s*(\\/(\\S)+)\\/"}],blockComment:[{regex:"\\*\\/",token:"multiline.comment.end.puppet",next:"pop"},{defaultToken:"comment"}],constants:[{token:"constant.language.puppet",regex:"\\b(false|true|running|stopped|installed|purged|latest|file|directory|held|undef|present|absent|link|mounted|unmounted)\\b"}],variable:[{token:"variable.puppet",regex:"(\\$[a-z0-9_{][a-zA-Z0-9_]*)"}],strings:[{token:"punctuation.quote.puppet",regex:"'",push:[{token:"punctuation.quote.puppet",regex:"'",next:"pop"},{include:"escaped_chars"},{defaultToken:"string"}]},{token:"punctuation.quote.puppet",regex:'"',push:[{token:"punctuation.quote.puppet",regex:'"',next:"pop"},{include:"escaped_chars"},{include:"variable"},{defaultToken:"string"}]}],escaped_chars:[{token:"constant.escaped_char.puppet",regex:"\\\\."}],operators:[{token:"keyword.operator",regex:"\\+\\.|\\-\\.|\\*\\.|\\/\\.|#|;;|\\+|\\-|\\*|\\*\\*\\/|\\/\\/|%|<<|>>|&|\\||\\^|~|<|>|<=|=>|==|!=|<>|<-|=|::|,"}]},this.normalizeRules()};r.inherits(s,i),t.PuppetHighlightRules=s}),ace.define("ace/mode/folding/cstyle",["require","exports","module","ace/lib/oop","ace/range","ace/mode/folding/fold_mode"],function(e,t,n){"use strict";var r=e("../../lib/oop"),i=e("../../range").Range,s=e("./fold_mode").FoldMode,o=t.FoldMode=function(e){e&&(this.foldingStartMarker=new RegExp(this.foldingStartMarker.source.replace(/\|[^|]*?$/,"|"+e.start)),this.foldingStopMarker=new RegExp(this.foldingStopMarker.source.replace(/\|[^|]*?$/,"|"+e.end)))};r.inherits(o,s),function(){this.foldingStartMarker=/([\{\[\(])[^\}\]\)]*$|^\s*(\/\*)/,this.foldingStopMarker=/^[^\[\{\(]*([\}\]\)])|^[\s\*]*(\*\/)/,this.singleLineBlockCommentRe=/^\s*(\/\*).*\*\/\s*$/,this.tripleStarBlockCommentRe=/^\s*(\/\*\*\*).*\*\/\s*$/,this.startRegionRe=/^\s*(\/\*|\/\/)#?region\b/,this._getFoldWidgetBase=this.getFoldWidget,this.getFoldWidget=function(e,t,n){var r=e.getLine(n);if(this.singleLineBlockCommentRe.test(r)&&!this.startRegionRe.test(r)&&!this.tripleStarBlockCommentRe.test(r))return"";var i=this._getFoldWidgetBase(e,t,n);return!i&&this.startRegionRe.test(r)?"start":i},this.getFoldWidgetRange=function(e,t,n,r){var i=e.getLine(n);if(this.startRegionRe.test(i))return this.getCommentRegionBlock(e,i,n);var s=i.match(this.foldingStartMarker);if(s){var o=s.index;if(s[1])return this.openingBracketBlock(e,s[1],n,o);var u=e.getCommentFoldRange(n,o+s[0].length,1);return u&&!u.isMultiLine()&&(r?u=this.getSectionRange(e,n):t!="all"&&(u=null)),u}if(t==="markbegin")return;var s=i.match(this.foldingStopMarker);if(s){var o=s.index+s[0].length;return s[1]?this.closingBracketBlock(e,s[1],n,o):e.getCommentFoldRange(n,o,-1)}},this.getSectionRange=function(e,t){var n=e.getLine(t),r=n.search(/\S/),s=t,o=n.length;t+=1;var u=t,a=e.getLength();while(++t<a){n=e.getLine(t);var f=n.search(/\S/);if(f===-1)continue;if(r>f)break;var l=this.getFoldWidgetRange(e,"all",t);if(l){if(l.start.row<=s)break;if(l.isMultiLine())t=l.end.row;else if(r==f)break}u=t}return new i(s,o,u,e.getLine(u).length)},this.getCommentRegionBlock=function(e,t,n){var r=t.search(/\s*$/),s=e.getLength(),o=n,u=/^\s*(?:\/\*|\/\/|--)#?(end)?region\b/,a=1;while(++n<s){t=e.getLine(n);var f=u.exec(t);if(!f)continue;f[1]?a--:a++;if(!a)break}var l=n;if(l>o)return new i(o,r,l,t.length)}}.call(o.prototype)}),ace.define("ace/mode/matching_brace_outdent",["require","exports","module","ace/range"],function(e,t,n){"use strict";var r=e("../range").Range,i=function(){};(function(){this.checkOutdent=function(e,t){return/^\s+$/.test(e)?/^\s*\}/.test(t):!1},this.autoOutdent=function(e,t){var n=e.getLine(t),i=n.match(/^(\s*\})/);if(!i)return 0;var s=i[1].length,o=e.findMatchingBracket({row:t,column:s});if(!o||o.row==t)return 0;var u=this.$getIndent(e.getLine(o.row));e.replace(new r(t,0,t,s-1),u)},this.$getIndent=function(e){return e.match(/^\s*/)[0]}}).call(i.prototype),t.MatchingBraceOutdent=i}),ace.define("ace/mode/puppet",["require","exports","module","ace/lib/oop","ace/mode/text","ace/mode/puppet_highlight_rules","ace/mode/behaviour/cstyle","ace/mode/folding/cstyle","ace/mode/matching_brace_outdent"],function(e,t,n){"use strict";var r=e("../lib/oop"),i=e("./text").Mode,s=e("./puppet_highlight_rules").PuppetHighlightRules,o=e("./behaviour/cstyle").CstyleBehaviour,u=e("./folding/cstyle").FoldMode,a=e("./matching_brace_outdent").MatchingBraceOutdent,f=function(){i.call(this),this.HighlightRules=s,this.$outdent=new a,this.$behaviour=new o,this.foldingRules=new u};r.inherits(f,i),function(){this.lineCommentStart="#",this.blockComment={start:"/*",end:"*/"},this.$id="ace/mode/puppet"}.call(f.prototype),t.Mode=f});                (function() {
                    ace.require(["ace/mode/puppet"], function(m) {
                        if (typeof module == "object" && typeof exports == "object" && module) {
                            module.exports = m;
                        }
                    });
                })();
            